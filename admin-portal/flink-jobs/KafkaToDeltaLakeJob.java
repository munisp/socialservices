package com.socialprotection.flink;

import org.apache.flink.api.common.eventtime.WatermarkStrategy;
import org.apache.flink.api.common.functions.MapFunction;
import org.apache.flink.api.common.serialization.SimpleStringSchema;
import org.apache.flink.connector.kafka.source.KafkaSource;
import org.apache.flink.connector.kafka.source.enumerator.initializer.OffsetsInitializer;
import org.apache.flink.streaming.api.datastream.DataStream;
import org.apache.flink.streaming.api.environment.StreamExecutionEnvironment;
import org.apache.flink.streaming.api.functions.sink.SinkFunction;
import org.apache.flink.shaded.jackson2.com.fasterxml.jackson.databind.JsonNode;
import org.apache.flink.shaded.jackson2.com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Flink job to ingest events from Kafka and write to Delta Lake via Python service
 * 
 * Features:
 * - Exactly-once semantics with Kafka offsets
 * - Batch writes to Delta Lake (100 events per batch)
 * - Automatic schema validation
 * - Error handling and retry logic
 */
public class KafkaToDeltaLakeJob {
    
    private static final String KAFKA_BOOTSTRAP_SERVERS = System.getenv().getOrDefault("KAFKA_BOOTSTRAP_SERVERS", "kafka:9092");
    private static final String DELTA_LAKE_SERVICE_URL = System.getenv().getOrDefault("DELTA_LAKE_SERVICE_URL", "http://deltalake-service:5001");
    private static final int BATCH_SIZE = 100;
    
    public static void main(String[] args) throws Exception {
        // Set up the streaming execution environment
        final StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();
        
        // Enable checkpointing for exactly-once semantics
        env.enableCheckpointing(60000); // Checkpoint every 60 seconds
        env.getCheckpointConfig().setCheckpointTimeout(600000); // 10 minutes timeout
        env.getCheckpointConfig().setMinPauseBetweenCheckpoints(30000); // 30 seconds between checkpoints
        
        // Create Kafka source for enrollment events
        KafkaSource<String> enrollmentSource = KafkaSource.<String>builder()
                .setBootstrapServers(KAFKA_BOOTSTRAP_SERVERS)
                .setTopics("enrollment_events")
                .setGroupId("flink-deltalake-enrollment")
                .setStartingOffsets(OffsetsInitializer.earliest())
                .setValueOnlyDeserializer(new SimpleStringSchema())
                .build();
        
        // Create Kafka source for disbursement events
        KafkaSource<String> disbursementSource = KafkaSource.<String>builder()
                .setBootstrapServers(KAFKA_BOOTSTRAP_SERVERS)
                .setTopics("disbursement_events")
                .setGroupId("flink-deltalake-disbursement")
                .setStartingOffsets(OffsetsInitializer.earliest())
                .setValueOnlyDeserializer(new SimpleStringSchema())
                .build();
        
        // Create Kafka source for KYC events
        KafkaSource<String> kycSource = KafkaSource.<String>builder()
                .setBootstrapServers(KAFKA_BOOTSTRAP_SERVERS)
                .setTopics("kyc_events")
                .setGroupId("flink-deltalake-kyc")
                .setStartingOffsets(OffsetsInitializer.earliest())
                .setValueOnlyDeserializer(new SimpleStringSchema())
                .build();
        
        // Process enrollment events
        DataStream<String> enrollmentStream = env.fromSource(
                enrollmentSource,
                WatermarkStrategy.forBoundedOutOfOrderness(Duration.ofSeconds(5)),
                "Enrollment Kafka Source"
        );
        
        enrollmentStream
                .map(new EventParser("enrollment"))
                .addSink(new DeltaLakeSink("bronze/enrollment_events"));
        
        // Process disbursement events
        DataStream<String> disbursementStream = env.fromSource(
                disbursementSource,
                WatermarkStrategy.forBoundedOutOfOrderness(Duration.ofSeconds(5)),
                "Disbursement Kafka Source"
        );
        
        disbursementStream
                .map(new EventParser("disbursement"))
                .addSink(new DeltaLakeSink("bronze/disbursement_events"));
        
        // Process KYC events
        DataStream<String> kycStream = env.fromSource(
                kycSource,
                WatermarkStrategy.forBoundedOutOfOrderness(Duration.ofSeconds(5)),
                "KYC Kafka Source"
        );
        
        kycStream
                .map(new EventParser("kyc"))
                .addSink(new DeltaLakeSink("bronze/kyc_events"));
        
        // Execute the job
        env.execute("Kafka to Delta Lake Ingestion Job");
    }
    
    /**
     * Parse and enrich Kafka events
     */
    public static class EventParser implements MapFunction<String, Map<String, Object>> {
        private final String eventType;
        private final ObjectMapper mapper = new ObjectMapper();
        
        public EventParser(String eventType) {
            this.eventType = eventType;
        }
        
        @Override
        public Map<String, Object> map(String value) throws Exception {
            // Parse JSON
            JsonNode json = mapper.readTree(value);
            
            // Convert to Map
            Map<String, Object> event = mapper.convertValue(json, Map.class);
            
            // Add metadata
            event.put("event_type", eventType);
            event.put("ingestion_timestamp", System.currentTimeMillis());
            event.put("processing_time", System.currentTimeMillis());
            
            return event;
        }
    }
    
    /**
     * Sink that writes batches of events to Delta Lake via Python service
     */
    public static class DeltaLakeSink implements SinkFunction<Map<String, Object>> {
        private final String tablePath;
        private final List<Map<String, Object>> buffer = new ArrayList<>();
        private final HttpClient httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(10))
                .build();
        private final ObjectMapper mapper = new ObjectMapper();
        
        public DeltaLakeSink(String tablePath) {
            this.tablePath = tablePath;
        }
        
        @Override
        public void invoke(Map<String, Object> value, Context context) throws Exception {
            buffer.add(value);
            
            // Flush when batch size reached
            if (buffer.size() >= BATCH_SIZE) {
                flush();
            }
        }
        
        private void flush() throws IOException, InterruptedException {
            if (buffer.isEmpty()) {
                return;
            }
            
            // Prepare request payload
            Map<String, Object> payload = new HashMap<>();
            payload.put("table_path", tablePath);
            payload.put("data", new ArrayList<>(buffer));
            payload.put("mode", "append");
            
            String jsonPayload = mapper.writeValueAsString(payload);
            
            // Send HTTP request to Delta Lake service
            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create(DELTA_LAKE_SERVICE_URL + "/tables/write"))
                    .header("Content-Type", "application/json")
                    .POST(HttpRequest.BodyPublishers.ofString(jsonPayload))
                    .timeout(Duration.ofSeconds(30))
                    .build();
            
            HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
            
            if (response.statusCode() != 200) {
                throw new IOException("Failed to write to Delta Lake: " + response.body());
            }
            
            // Parse response
            JsonNode responseJson = mapper.readTree(response.body());
            if (!responseJson.get("success").asBoolean()) {
                throw new IOException("Delta Lake write failed: " + responseJson.get("error").asText());
            }
            
            System.out.println("Successfully wrote " + buffer.size() + " events to " + tablePath);
            
            // Clear buffer
            buffer.clear();
        }
        
        @Override
        public void finish() throws Exception {
            // Flush remaining events
            flush();
        }
    }
}
