name := "spark-deltalake-jobs"

version := "1.0.0"

scalaVersion := "2.12.18"

libraryDependencies ++= Seq(
  // Apache Spark
  "org.apache.spark" %% "spark-core" % "3.5.0" % "provided",
  "org.apache.spark" %% "spark-sql" % "3.5.0" % "provided",
  
  // Delta Lake
  "io.delta" %% "delta-core" % "3.0.0",
  
  // AWS S3 support
  "org.apache.hadoop" % "hadoop-aws" % "3.3.4",
  "com.amazonaws" % "aws-java-sdk-bundle" % "1.12.262",
  
  // Logging
  "org.slf4j" % "slf4j-api" % "1.7.36" % "provided",
  "org.apache.logging.log4j" % "log4j-slf4j-impl" % "2.20.0" % "provided"
)

// Assembly settings for creating fat JAR
assembly / assemblyMergeStrategy := {
  case PathList("META-INF", xs @ _*) => MergeStrategy.discard
  case "reference.conf" => MergeStrategy.concat
  case x => MergeStrategy.first
}

assembly / assemblyJarName := "spark-deltalake-jobs.jar"

// Exclude Spark and Hadoop from assembly (provided by cluster)
assembly / assemblyExcludedJars := {
  val cp = (assembly / fullClasspath).value
  cp.filter { f =>
    f.data.getName.contains("spark-") ||
    f.data.getName.contains("hadoop-") ||
    f.data.getName.contains("scala-library")
  }
}
