package com.socialprotection.spark

import org.apache.spark.sql.{DataFrame, SparkSession}
import org.apache.spark.sql.functions._
import org.apache.spark.sql.expressions.Window
import io.delta.tables._

/**
 * Spark job to create Gold layer aggregations from Silver layer
 * 
 * Features:
 * - Program-level metrics and KPIs
 * - Beneficiary analytics
 * - Disbursement analytics
 * - Time-series aggregations
 * - Geospatial aggregations
 */
object SilverToGoldAggregation {
  
  def main(args: Array[String]): Unit = {
    // Create Spark session with Delta Lake support
    val spark = SparkSession.builder()
      .appName("Silver to Gold Aggregation")
      .config("spark.sql.extensions", "io.delta.sql.DeltaSparkSessionExtension")
      .config("spark.sql.catalog.spark_catalog", "org.apache.spark.sql.delta.catalog.DeltaCatalog")
      .getOrCreate()
    
    import spark.implicits._
    
    val lakehousePath = sys.env.getOrElse("LAKEHOUSE_PATH", "s3://lakehouse")
    
    // Create program-level aggregations
    createProgramMetrics(spark, lakehousePath)
    
    // Create beneficiary analytics
    createBeneficiaryAnalytics(spark, lakehousePath)
    
    // Create disbursement analytics
    createDisbursementAnalytics(spark, lakehousePath)
    
    // Create time-series aggregations
    createTimeSeriesMetrics(spark, lakehousePath)
    
    spark.stop()
  }
  
  /**
   * Create program-level metrics and KPIs
   */
  def createProgramMetrics(spark: SparkSession, lakehousePath: String): Unit = {
    println("Creating program metrics...")
    
    // Read Silver tables
    val enrollments = spark.read.format("delta").load(s"$lakehousePath/silver/enrollments")
    val disbursements = spark.read.format("delta").load(s"$lakehousePath/silver/disbursements")
    val kycVerifications = spark.read.format("delta").load(s"$lakehousePath/silver/kyc_verifications")
    
    // Calculate program metrics
    val programMetrics = enrollments
      .groupBy("program_id")
      .agg(
        count("*").as("total_enrollments"),
        countDistinct("beneficiary_id").as("unique_beneficiaries"),
        count(when($"status" === "ACTIVE", 1)).as("active_enrollments"),
        count(when($"status" === "PENDING", 1)).as("pending_enrollments"),
        count(when($"status" === "REJECTED", 1)).as("rejected_enrollments"),
        min("enrollment_date").as("first_enrollment_date"),
        max("enrollment_date").as("last_enrollment_date")
      )
      // Join with disbursement metrics
      .join(
        disbursements
          .groupBy("program_id")
          .agg(
            count("*").as("total_disbursements"),
            sum("amount").as("total_disbursed_amount"),
            avg("amount").as("avg_disbursement_amount"),
            count(when($"status" === "COMPLETED", 1)).as("completed_disbursements"),
            count(when($"status" === "FAILED", 1)).as("failed_disbursements")
          ),
        Seq("program_id"),
        "left"
      )
      // Join with KYC metrics
      .join(
        kycVerifications
          .join(enrollments.select("beneficiary_id", "program_id"), Seq("beneficiary_id"))
          .groupBy("program_id")
          .agg(
            count("*").as("total_kyc_checks"),
            count(when($"status" === "VERIFIED", 1)).as("verified_kyc"),
            count(when($"status" === "REJECTED", 1)).as("rejected_kyc")
          ),
        Seq("program_id"),
        "left"
      )
      // Calculate derived metrics
      .withColumn("enrollment_approval_rate", 
        when($"total_enrollments" > 0, 
          $"active_enrollments" / $"total_enrollments" * 100
        ).otherwise(0))
      .withColumn("disbursement_success_rate",
        when($"total_disbursements" > 0,
          $"completed_disbursements" / $"total_disbursements" * 100
        ).otherwise(0))
      .withColumn("kyc_verification_rate",
        when($"total_kyc_checks" > 0,
          $"verified_kyc" / $"total_kyc_checks" * 100
        ).otherwise(0))
      .withColumn("updated_at", current_timestamp())
    
    // Write to Gold Delta table
    val goldPath = s"$lakehousePath/gold/program_metrics"
    programMetrics.write
      .format("delta")
      .mode("overwrite")
      .save(goldPath)
    
    println(s"Created metrics for ${programMetrics.count()} programs")
  }
  
  /**
   * Create beneficiary analytics
   */
  def createBeneficiaryAnalytics(spark: SparkSession, lakehousePath: String): Unit = {
    println("Creating beneficiary analytics...")
    
    // Read Silver tables
    val enrollments = spark.read.format("delta").load(s"$lakehousePath/silver/enrollments")
    val disbursements = spark.read.format("delta").load(s"$lakehousePath/silver/disbursements")
    val kycVerifications = spark.read.format("delta").load(s"$lakehousePath/silver/kyc_verifications")
    
    // Calculate beneficiary analytics
    val beneficiaryAnalytics = enrollments
      .select("beneficiary_id", "program_id", "name", "location", "latitude", "longitude", "enrollment_date", "status")
      // Join with disbursement summary
      .join(
        disbursements
          .groupBy("beneficiary_id")
          .agg(
            count("*").as("total_disbursements"),
            sum("amount").as("total_received_amount"),
            avg("amount").as("avg_disbursement_amount"),
            min("disbursement_date").as("first_disbursement_date"),
            max("disbursement_date").as("last_disbursement_date"),
            count(when($"status" === "COMPLETED", 1)).as("completed_disbursements"),
            count(when($"status" === "FAILED", 1)).as("failed_disbursements")
          ),
        Seq("beneficiary_id"),
        "left"
      )
      // Join with KYC status
      .join(
        kycVerifications
          .withColumn("row_num", row_number().over(
            Window.partitionBy("beneficiary_id").orderBy(desc("verification_date"))
          ))
          .filter($"row_num" === 1)
          .select("beneficiary_id", "status".as("kyc_status"), "verification_date".as("kyc_verification_date")),
        Seq("beneficiary_id"),
        "left"
      )
      // Calculate derived metrics
      .withColumn("days_since_enrollment", 
        datediff(current_date(), $"enrollment_date"))
      .withColumn("days_since_last_disbursement",
        when($"last_disbursement_date".isNotNull,
          datediff(current_date(), $"last_disbursement_date")
        ).otherwise(null))
      .withColumn("disbursement_frequency_days",
        when($"total_disbursements" > 1,
          datediff($"last_disbursement_date", $"first_disbursement_date") / ($"total_disbursements" - 1)
        ).otherwise(null))
      .withColumn("is_kyc_verified", $"kyc_status" === "VERIFIED")
      .withColumn("is_active", $"status" === "ACTIVE")
      .withColumn("updated_at", current_timestamp())
    
    // Write to Gold Delta table
    val goldPath = s"$lakehousePath/gold/beneficiary_analytics"
    beneficiaryAnalytics.write
      .format("delta")
      .mode("overwrite")
      .partitionBy("program_id")
      .save(goldPath)
    
    // Optimize and Z-order
    spark.sql(s"OPTIMIZE delta.`$goldPath` ZORDER BY (beneficiary_id, total_received_amount)")
    
    println(s"Created analytics for ${beneficiaryAnalytics.count()} beneficiaries")
  }
  
  /**
   * Create disbursement analytics
   */
  def createDisbursementAnalytics(spark: SparkSession, lakehousePath: String): Unit = {
    println("Creating disbursement analytics...")
    
    // Read Silver tables
    val disbursements = spark.read.format("delta").load(s"$lakehousePath/silver/disbursements")
    
    // Calculate disbursement analytics by program and date
    val disbursementAnalytics = disbursements
      .withColumn("disbursement_month", date_trunc("month", $"disbursement_date"))
      .groupBy("program_id", "disbursement_month", "currency", "payment_method")
      .agg(
        count("*").as("total_transactions"),
        countDistinct("beneficiary_id").as("unique_beneficiaries"),
        sum("amount").as("total_amount"),
        avg("amount").as("avg_amount"),
        min("amount").as("min_amount"),
        max("amount").as("max_amount"),
        stddev("amount").as("stddev_amount"),
        count(when($"status" === "COMPLETED", 1)).as("completed_count"),
        count(when($"status" === "FAILED", 1)).as("failed_count"),
        count(when($"status" === "PENDING", 1)).as("pending_count")
      )
      // Calculate derived metrics
      .withColumn("success_rate",
        when($"total_transactions" > 0,
          $"completed_count" / $"total_transactions" * 100
        ).otherwise(0))
      .withColumn("failure_rate",
        when($"total_transactions" > 0,
          $"failed_count" / $"total_transactions" * 100
        ).otherwise(0))
      .withColumn("updated_at", current_timestamp())
    
    // Write to Gold Delta table
    val goldPath = s"$lakehousePath/gold/disbursement_analytics"
    disbursementAnalytics.write
      .format("delta")
      .mode("overwrite")
      .partitionBy("program_id", "disbursement_month")
      .save(goldPath)
    
    println(s"Created ${disbursementAnalytics.count()} disbursement analytics records")
  }
  
  /**
   * Create time-series metrics for dashboards
   */
  def createTimeSeriesMetrics(spark: SparkSession, lakehousePath: String): Unit = {
    println("Creating time-series metrics...")
    
    // Read Silver tables
    val enrollments = spark.read.format("delta").load(s"$lakehousePath/silver/enrollments")
    val disbursements = spark.read.format("delta").load(s"$lakehousePath/silver/disbursements")
    
    // Daily enrollment metrics
    val dailyEnrollments = enrollments
      .withColumn("date", to_date($"enrollment_date"))
      .groupBy("program_id", "date")
      .agg(
        count("*").as("enrollments_count"),
        count(when($"status" === "ACTIVE", 1)).as("active_count"),
        count(when($"status" === "PENDING", 1)).as("pending_count")
      )
      .withColumn("metric_type", lit("enrollment"))
      .withColumn("updated_at", current_timestamp())
    
    // Daily disbursement metrics
    val dailyDisbursements = disbursements
      .withColumn("date", to_date($"disbursement_date"))
      .groupBy("program_id", "date")
      .agg(
        count("*").as("disbursements_count"),
        sum("amount").as("total_amount"),
        count(when($"status" === "COMPLETED", 1)).as("completed_count")
      )
      .withColumn("metric_type", lit("disbursement"))
      .withColumn("updated_at", current_timestamp())
    
    // Combine metrics
    val timeSeriesMetrics = dailyEnrollments
      .join(dailyDisbursements, Seq("program_id", "date"), "outer")
      .na.fill(0)
      .withColumn("updated_at", current_timestamp())
    
    // Write to Gold Delta table
    val goldPath = s"$lakehousePath/gold/time_series_metrics"
    timeSeriesMetrics.write
      .format("delta")
      .mode("overwrite")
      .partitionBy("program_id", "date")
      .save(goldPath)
    
    println(s"Created ${timeSeriesMetrics.count()} time-series metric records")
  }
}
