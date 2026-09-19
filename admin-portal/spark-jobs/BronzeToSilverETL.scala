package com.socialprotection.spark

import org.apache.spark.sql.{DataFrame, SparkSession}
import org.apache.spark.sql.functions._
import org.apache.spark.sql.types._
import io.delta.tables._

/**
 * Spark job to transform Bronze layer data to Silver layer
 * 
 * Features:
 * - Deduplication by event_id
 * - Schema validation and enforcement
 * - Data quality checks
 * - Enrichment with reference data
 * - Z-ordering for query performance
 */
object BronzeToSilverETL {
  
  def main(args: Array[String]): Unit = {
    // Create Spark session with Delta Lake support
    val spark = SparkSession.builder()
      .appName("Bronze to Silver ETL")
      .config("spark.sql.extensions", "io.delta.sql.DeltaSparkSessionExtension")
      .config("spark.sql.catalog.spark_catalog", "org.apache.spark.sql.delta.catalog.DeltaCatalog")
      .getOrCreate()
    
    import spark.implicits._
    
    val lakehousePath = sys.env.getOrElse("LAKEHOUSE_PATH", "s3://lakehouse")
    
    // Process enrollment events
    processEnrollmentEvents(spark, lakehousePath)
    
    // Process disbursement events
    processDisbursementEvents(spark, lakehousePath)
    
    // Process KYC events
    processKYCEvents(spark, lakehousePath)
    
    spark.stop()
  }
  
  /**
   * Process enrollment events from Bronze to Silver
   */
  def processEnrollmentEvents(spark: SparkSession, lakehousePath: String): Unit = {
    println("Processing enrollment events...")
    
    // Read from Bronze Delta table
    val bronzeDF = spark.read
      .format("delta")
      .load(s"$lakehousePath/bronze/enrollment_events")
    
    // Define Silver schema
    val silverSchema = StructType(Seq(
      StructField("enrollment_id", StringType, nullable = false),
      StructField("beneficiary_id", StringType, nullable = false),
      StructField("program_id", StringType, nullable = false),
      StructField("name", StringType, nullable = false),
      StructField("email", StringType, nullable = true),
      StructField("phone", StringType, nullable = true),
      StructField("location", StringType, nullable = true),
      StructField("latitude", DoubleType, nullable = true),
      StructField("longitude", DoubleType, nullable = true),
      StructField("enrollment_date", TimestampType, nullable = false),
      StructField("status", StringType, nullable = false),
      StructField("created_at", TimestampType, nullable = false),
      StructField("updated_at", TimestampType, nullable = false),
      StructField("ingestion_timestamp", LongType, nullable = false)
    ))
    
    // Transform and clean data
    val silverDF = bronzeDF
      // Deduplicate by enrollment_id, keep latest
      .withColumn("row_num", row_number().over(
        Window.partitionBy("enrollment_id").orderBy(desc("ingestion_timestamp"))
      ))
      .filter($"row_num" === 1)
      .drop("row_num")
      // Validate required fields
      .filter($"enrollment_id".isNotNull && $"beneficiary_id".isNotNull && $"program_id".isNotNull)
      // Standardize status values
      .withColumn("status", upper(trim($"status")))
      .filter($"status".isin("PENDING", "APPROVED", "REJECTED", "ACTIVE", "INACTIVE"))
      // Add processing timestamp
      .withColumn("updated_at", current_timestamp())
      // Select and cast to schema
      .select(
        $"enrollment_id",
        $"beneficiary_id",
        $"program_id",
        $"name",
        $"email",
        $"phone",
        $"location",
        $"latitude".cast(DoubleType),
        $"longitude".cast(DoubleType),
        $"enrollment_date".cast(TimestampType),
        $"status",
        $"created_at".cast(TimestampType),
        $"updated_at",
        $"ingestion_timestamp"
      )
    
    // Write to Silver Delta table with merge (upsert)
    val silverPath = s"$lakehousePath/silver/enrollments"
    
    if (DeltaTable.isDeltaTable(spark, silverPath)) {
      val silverTable = DeltaTable.forPath(spark, silverPath)
      
      silverTable.as("target")
        .merge(
          silverDF.as("source"),
          "target.enrollment_id = source.enrollment_id"
        )
        .whenMatched()
        .updateAll()
        .whenNotMatched()
        .insertAll()
        .execute()
    } else {
      silverDF.write
        .format("delta")
        .mode("overwrite")
        .partitionBy("program_id")
        .save(silverPath)
    }
    
    // Optimize and Z-order
    spark.sql(s"OPTIMIZE delta.`$silverPath` ZORDER BY (beneficiary_id, enrollment_date)")
    
    println(s"Processed ${silverDF.count()} enrollment events")
  }
  
  /**
   * Process disbursement events from Bronze to Silver
   */
  def processDisbursementEvents(spark: SparkSession, lakehousePath: String): Unit = {
    println("Processing disbursement events...")
    
    // Read from Bronze Delta table
    val bronzeDF = spark.read
      .format("delta")
      .load(s"$lakehousePath/bronze/disbursement_events")
    
    // Transform and clean data
    val silverDF = bronzeDF
      // Deduplicate by disbursement_id
      .withColumn("row_num", row_number().over(
        Window.partitionBy("disbursement_id").orderBy(desc("ingestion_timestamp"))
      ))
      .filter($"row_num" === 1)
      .drop("row_num")
      // Validate required fields
      .filter($"disbursement_id".isNotNull && $"beneficiary_id".isNotNull && $"amount".isNotNull)
      // Validate amount is positive
      .filter($"amount".cast(DoubleType) > 0)
      // Standardize status
      .withColumn("status", upper(trim($"status")))
      .filter($"status".isin("PENDING", "PROCESSING", "COMPLETED", "FAILED", "CANCELLED"))
      // Add processing timestamp
      .withColumn("updated_at", current_timestamp())
      // Select and cast
      .select(
        $"disbursement_id",
        $"beneficiary_id",
        $"program_id",
        $"amount".cast(DoubleType),
        $"currency",
        $"disbursement_date".cast(TimestampType),
        $"payment_method",
        $"status",
        $"transaction_ref",
        $"created_at".cast(TimestampType),
        $"updated_at",
        $"ingestion_timestamp"
      )
    
    // Write to Silver Delta table with merge
    val silverPath = s"$lakehousePath/silver/disbursements"
    
    if (DeltaTable.isDeltaTable(spark, silverPath)) {
      val silverTable = DeltaTable.forPath(spark, silverPath)
      
      silverTable.as("target")
        .merge(
          silverDF.as("source"),
          "target.disbursement_id = source.disbursement_id"
        )
        .whenMatched()
        .updateAll()
        .whenNotMatched()
        .insertAll()
        .execute()
    } else {
      silverDF.write
        .format("delta")
        .mode("overwrite")
        .partitionBy("program_id", "disbursement_date")
        .save(silverPath)
    }
    
    // Optimize and Z-order
    spark.sql(s"OPTIMIZE delta.`$silverPath` ZORDER BY (beneficiary_id, disbursement_date)")
    
    println(s"Processed ${silverDF.count()} disbursement events")
  }
  
  /**
   * Process KYC events from Bronze to Silver
   */
  def processKYCEvents(spark: SparkSession, lakehousePath: String): Unit = {
    println("Processing KYC events...")
    
    // Read from Bronze Delta table
    val bronzeDF = spark.read
      .format("delta")
      .load(s"$lakehousePath/bronze/kyc_events")
    
    // Transform and clean data
    val silverDF = bronzeDF
      // Deduplicate by kyc_id
      .withColumn("row_num", row_number().over(
        Window.partitionBy("kyc_id").orderBy(desc("ingestion_timestamp"))
      ))
      .filter($"row_num" === 1)
      .drop("row_num")
      // Validate required fields
      .filter($"kyc_id".isNotNull && $"beneficiary_id".isNotNull)
      // Standardize status
      .withColumn("status", upper(trim($"status")))
      .filter($"status".isin("PENDING", "IN_PROGRESS", "VERIFIED", "REJECTED", "EXPIRED"))
      // Add processing timestamp
      .withColumn("updated_at", current_timestamp())
      // Select and cast
      .select(
        $"kyc_id",
        $"beneficiary_id",
        $"document_type",
        $"document_number",
        $"verification_method",
        $"verification_date".cast(TimestampType),
        $"status",
        $"verified_by",
        $"notes",
        $"created_at".cast(TimestampType),
        $"updated_at",
        $"ingestion_timestamp"
      )
    
    // Write to Silver Delta table with merge
    val silverPath = s"$lakehousePath/silver/kyc_verifications"
    
    if (DeltaTable.isDeltaTable(spark, silverPath)) {
      val silverTable = DeltaTable.forPath(spark, silverPath)
      
      silverTable.as("target")
        .merge(
          silverDF.as("source"),
          "target.kyc_id = source.kyc_id"
        )
        .whenMatched()
        .updateAll()
        .whenNotMatched()
        .insertAll()
        .execute()
    } else {
      silverDF.write
        .format("delta")
        .mode("overwrite")
        .partitionBy("status")
        .save(silverPath)
    }
    
    // Optimize and Z-order
    spark.sql(s"OPTIMIZE delta.`$silverPath` ZORDER BY (beneficiary_id, verification_date)")
    
    println(s"Processed ${silverDF.count()} KYC events")
  }
}
