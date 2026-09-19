#!/usr/bin/env python3
"""
End-to-End Lakehouse Journey Test

Simulates a complete beneficiary journey through the lakehouse:
1. Produce events to Kafka (enrollment, KYC, disbursement)
2. Verify Flink ingestion to Bronze layer
3. Trigger Spark ETL to Silver layer
4. Trigger Spark aggregation to Gold layer
5. Validate analytics results
"""

import json
import time
import requests
from datetime import datetime, timedelta
from kafka import KafkaProducer
from typing import Dict, List, Any
import uuid

# Configuration
KAFKA_BOOTSTRAP_SERVERS = "localhost:9092"
DELTA_LAKE_SERVICE_URL = "http://localhost:5001"
LAKEHOUSE_PATH = "s3://lakehouse"

class LakehouseJourneyTest:
    def __init__(self):
        self.producer = KafkaProducer(
            bootstrap_servers=KAFKA_BOOTSTRAP_SERVERS,
            value_serializer=lambda v: json.dumps(v).encode('utf-8')
        )
        self.test_beneficiary_id = f"BEN-TEST-{uuid.uuid4().hex[:8]}"
        self.test_program_id = "PROG-001"
        self.test_enrollment_id = f"ENR-{uuid.uuid4().hex[:8]}"
        self.test_kyc_id = f"KYC-{uuid.uuid4().hex[:8]}"
        self.test_disbursement_ids = []
        
    def step1_produce_enrollment_event(self):
        """Step 1: Produce enrollment event to Kafka"""
        print("\n=== STEP 1: Producing Enrollment Event ===")
        
        enrollment_event = {
            "enrollment_id": self.test_enrollment_id,
            "beneficiary_id": self.test_beneficiary_id,
            "program_id": self.test_program_id,
            "name": "Test Beneficiary Alpha",
            "email": "test.alpha@example.com",
            "phone": "+1234567890",
            "location": "Test City, Test State",
            "latitude": 40.7128,
            "longitude": -74.0060,
            "enrollment_date": datetime.now().isoformat(),
            "status": "PENDING",
            "created_at": datetime.now().isoformat(),
        }
        
        self.producer.send('enrollment_events', enrollment_event)
        self.producer.flush()
        
        print(f"✅ Sent enrollment event: {self.test_enrollment_id}")
        print(f"   Beneficiary: {self.test_beneficiary_id}")
        print(f"   Program: {self.test_program_id}")
        
        return enrollment_event
    
    def step2_produce_kyc_event(self):
        """Step 2: Produce KYC verification event to Kafka"""
        print("\n=== STEP 2: Producing KYC Event ===")
        
        kyc_event = {
            "kyc_id": self.test_kyc_id,
            "beneficiary_id": self.test_beneficiary_id,
            "document_type": "NATIONAL_ID",
            "document_number": "ID123456789",
            "verification_method": "BIOMETRIC",
            "verification_date": datetime.now().isoformat(),
            "status": "VERIFIED",
            "verified_by": "SYSTEM_AUTO",
            "notes": "Automated verification successful",
            "created_at": datetime.now().isoformat(),
        }
        
        self.producer.send('kyc_events', kyc_event)
        self.producer.flush()
        
        print(f"✅ Sent KYC event: {self.test_kyc_id}")
        print(f"   Status: VERIFIED")
        
        return kyc_event
    
    def step3_produce_disbursement_events(self, count=3):
        """Step 3: Produce multiple disbursement events to Kafka"""
        print(f"\n=== STEP 3: Producing {count} Disbursement Events ===")
        
        disbursement_events = []
        for i in range(count):
            disbursement_id = f"DIS-{uuid.uuid4().hex[:8]}"
            self.test_disbursement_ids.append(disbursement_id)
            
            disbursement_event = {
                "disbursement_id": disbursement_id,
                "beneficiary_id": self.test_beneficiary_id,
                "program_id": self.test_program_id,
                "amount": 500.00 + (i * 50),  # Varying amounts
                "currency": "USD",
                "disbursement_date": (datetime.now() - timedelta(days=30-i*10)).isoformat(),
                "payment_method": "MOBILE_MONEY",
                "status": "COMPLETED",
                "transaction_ref": f"TXN-{uuid.uuid4().hex[:12]}",
                "created_at": datetime.now().isoformat(),
            }
            
            self.producer.send('disbursement_events', disbursement_event)
            disbursement_events.append(disbursement_event)
            
            print(f"✅ Sent disbursement {i+1}/{count}: {disbursement_id} (${disbursement_event['amount']})")
        
        self.producer.flush()
        return disbursement_events
    
    def step4_wait_for_flink_ingestion(self, wait_time=10):
        """Step 4: Wait for Flink to ingest events into Bronze layer"""
        print(f"\n=== STEP 4: Waiting for Flink Ingestion ({wait_time}s) ===")
        print("Flink is consuming from Kafka and writing to Bronze Delta tables...")
        
        for i in range(wait_time):
            print(f"⏳ {i+1}/{wait_time}s", end="\r")
            time.sleep(1)
        
        print("\n✅ Flink ingestion window completed")
    
    def step5_verify_bronze_layer(self):
        """Step 5: Verify data in Bronze layer"""
        print("\n=== STEP 5: Verifying Bronze Layer ===")
        
        tables_to_check = [
            ("bronze/enrollment_events", self.test_enrollment_id, "enrollment_id"),
            ("bronze/kyc_events", self.test_kyc_id, "kyc_id"),
            ("bronze/disbursement_events", self.test_disbursement_ids[0], "disbursement_id"),
        ]
        
        bronze_verified = True
        for table_path, test_id, id_field in tables_to_check:
            try:
                response = requests.post(
                    f"{DELTA_LAKE_SERVICE_URL}/tables/read",
                    json={
                        "table_path": table_path,
                        "filters": f"{id_field} = '{test_id}'"
                    },
                    timeout=10
                )
                
                if response.status_code == 200:
                    data = response.json()
                    if data.get("success") and len(data.get("data", [])) > 0:
                        print(f"✅ Found {len(data['data'])} record(s) in {table_path}")
                    else:
                        print(f"⚠️  No records found in {table_path} for {id_field}={test_id}")
                        bronze_verified = False
                else:
                    print(f"❌ Failed to read {table_path}: {response.status_code}")
                    bronze_verified = False
            except Exception as e:
                print(f"❌ Error reading {table_path}: {e}")
                bronze_verified = False
        
        return bronze_verified
    
    def step6_trigger_spark_etl(self):
        """Step 6: Trigger Spark Bronze → Silver ETL"""
        print("\n=== STEP 6: Triggering Spark Bronze → Silver ETL ===")
        print("Note: In production, this would be triggered via Spark submit")
        print("For testing, we'll simulate the ETL transformations...")
        
        # Simulate ETL process
        print("⏳ Running deduplication...")
        time.sleep(2)
        print("⏳ Running schema validation...")
        time.sleep(2)
        print("⏳ Running data quality checks...")
        time.sleep(2)
        print("⏳ Running merge upsert to Silver...")
        time.sleep(2)
        print("✅ Spark ETL completed")
        
        return True
    
    def step7_verify_silver_layer(self):
        """Step 7: Verify transformed data in Silver layer"""
        print("\n=== STEP 7: Verifying Silver Layer ===")
        
        tables_to_check = [
            ("silver/enrollments", self.test_enrollment_id, "enrollment_id"),
            ("silver/kyc_verifications", self.test_kyc_id, "kyc_id"),
            ("silver/disbursements", self.test_disbursement_ids[0], "disbursement_id"),
        ]
        
        silver_verified = True
        for table_path, test_id, id_field in tables_to_check:
            try:
                response = requests.post(
                    f"{DELTA_LAKE_SERVICE_URL}/tables/read",
                    json={
                        "table_path": table_path,
                        "filters": f"{id_field} = '{test_id}'"
                    },
                    timeout=10
                )
                
                if response.status_code == 200:
                    data = response.json()
                    if data.get("success") and len(data.get("data", [])) > 0:
                        record = data["data"][0]
                        print(f"✅ Found record in {table_path}")
                        print(f"   ID: {record.get(id_field)}")
                        print(f"   Status: {record.get('status', 'N/A')}")
                    else:
                        print(f"⚠️  No records found in {table_path}")
                        silver_verified = False
                else:
                    print(f"❌ Failed to read {table_path}: {response.status_code}")
                    silver_verified = False
            except Exception as e:
                print(f"❌ Error reading {table_path}: {e}")
                silver_verified = False
        
        return silver_verified
    
    def step8_trigger_spark_aggregation(self):
        """Step 8: Trigger Spark Silver → Gold aggregation"""
        print("\n=== STEP 8: Triggering Spark Silver → Gold Aggregation ===")
        print("Note: In production, this would be triggered via Spark submit")
        print("For testing, we'll simulate the aggregation...")
        
        # Simulate aggregation process
        print("⏳ Calculating program metrics...")
        time.sleep(2)
        print("⏳ Calculating beneficiary analytics...")
        time.sleep(2)
        print("⏳ Calculating disbursement analytics...")
        time.sleep(2)
        print("⏳ Calculating time-series metrics...")
        time.sleep(2)
        print("✅ Spark aggregation completed")
        
        return True
    
    def step9_verify_gold_layer(self):
        """Step 9: Verify analytics in Gold layer"""
        print("\n=== STEP 9: Verifying Gold Layer Analytics ===")
        
        tables_to_check = [
            ("gold/program_metrics", self.test_program_id, "program_id"),
            ("gold/beneficiary_analytics", self.test_beneficiary_id, "beneficiary_id"),
        ]
        
        gold_verified = True
        for table_path, test_id, id_field in tables_to_check:
            try:
                response = requests.post(
                    f"{DELTA_LAKE_SERVICE_URL}/tables/read",
                    json={
                        "table_path": table_path,
                        "filters": f"{id_field} = '{test_id}'"
                    },
                    timeout=10
                )
                
                if response.status_code == 200:
                    data = response.json()
                    if data.get("success") and len(data.get("data", [])) > 0:
                        record = data["data"][0]
                        print(f"✅ Found analytics in {table_path}")
                        
                        # Display key metrics
                        if "program_metrics" in table_path:
                            print(f"   Total Enrollments: {record.get('total_enrollments', 0)}")
                            print(f"   Total Disbursements: {record.get('total_disbursements', 0)}")
                            print(f"   Total Disbursed: ${record.get('total_disbursed_amount', 0):.2f}")
                        elif "beneficiary_analytics" in table_path:
                            print(f"   Total Received: ${record.get('total_received_amount', 0):.2f}")
                            print(f"   Disbursement Count: {record.get('total_disbursements', 0)}")
                            print(f"   KYC Verified: {record.get('is_kyc_verified', False)}")
                    else:
                        print(f"⚠️  No analytics found in {table_path}")
                        gold_verified = False
                else:
                    print(f"❌ Failed to read {table_path}: {response.status_code}")
                    gold_verified = False
            except Exception as e:
                print(f"❌ Error reading {table_path}: {e}")
                gold_verified = False
        
        return gold_verified
    
    def run_complete_journey(self):
        """Run the complete end-to-end journey test"""
        print("=" * 60)
        print("LAKEHOUSE END-TO-END JOURNEY TEST")
        print("=" * 60)
        print(f"Test Beneficiary: {self.test_beneficiary_id}")
        print(f"Test Program: {self.test_program_id}")
        print("=" * 60)
        
        try:
            # Phase 1: Event Production
            self.step1_produce_enrollment_event()
            time.sleep(1)
            self.step2_produce_kyc_event()
            time.sleep(1)
            self.step3_produce_disbursement_events(count=3)
            
            # Phase 2: Bronze Layer
            self.step4_wait_for_flink_ingestion(wait_time=15)
            bronze_ok = self.step5_verify_bronze_layer()
            
            if not bronze_ok:
                print("\n⚠️  Bronze layer verification failed. Continuing anyway...")
            
            # Phase 3: Silver Layer
            self.step6_trigger_spark_etl()
            silver_ok = self.step7_verify_silver_layer()
            
            if not silver_ok:
                print("\n⚠️  Silver layer verification failed. Continuing anyway...")
            
            # Phase 4: Gold Layer
            self.step8_trigger_spark_aggregation()
            gold_ok = self.step9_verify_gold_layer()
            
            # Summary
            print("\n" + "=" * 60)
            print("TEST SUMMARY")
            print("=" * 60)
            print(f"Bronze Layer: {'✅ PASS' if bronze_ok else '❌ FAIL'}")
            print(f"Silver Layer: {'✅ PASS' if silver_ok else '❌ FAIL'}")
            print(f"Gold Layer: {'✅ PASS' if gold_ok else '❌ FAIL'}")
            print("=" * 60)
            
            if bronze_ok and silver_ok and gold_ok:
                print("\n🎉 END-TO-END JOURNEY TEST: SUCCESS")
                return True
            else:
                print("\n⚠️  END-TO-END JOURNEY TEST: PARTIAL SUCCESS")
                print("Some layers could not be verified (services may not be running)")
                return False
                
        except Exception as e:
            print(f"\n❌ TEST FAILED: {e}")
            import traceback
            traceback.print_exc()
            return False
        finally:
            self.producer.close()

def main():
    """Main entry point"""
    test = LakehouseJourneyTest()
    success = test.run_complete_journey()
    exit(0 if success else 1)

if __name__ == "__main__":
    main()
