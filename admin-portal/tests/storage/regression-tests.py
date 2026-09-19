#!/usr/bin/env python3
"""
RustFS Storage Regression Test Suite

This test suite validates S3-compatible storage operations after migrating from MinIO to RustFS.
Tests cover all critical storage operations used by the social protection platform.
"""

import os
import sys
import json
import time
import hashlib
import tempfile
import unittest
from datetime import datetime, timedelta
from typing import Optional, List, Dict, Any
from io import BytesIO

import boto3
from botocore.config import Config
from botocore.exceptions import ClientError

# Configuration
S3_ENDPOINT = os.getenv("S3_ENDPOINT", "http://localhost:9000")
S3_ACCESS_KEY = os.getenv("S3_ACCESS_KEY", "rustfsadmin")
S3_SECRET_KEY = os.getenv("S3_SECRET_KEY", "rustfsadmin")
S3_REGION = os.getenv("S3_REGION", "us-east-1")
TEST_BUCKET = os.getenv("TEST_BUCKET", "regression-test-bucket")


def get_s3_client():
    """Create S3 client configured for RustFS/MinIO"""
    return boto3.client(
        "s3",
        endpoint_url=S3_ENDPOINT,
        aws_access_key_id=S3_ACCESS_KEY,
        aws_secret_access_key=S3_SECRET_KEY,
        region_name=S3_REGION,
        config=Config(
            signature_version="s3v4",
            s3={"addressing_style": "path"},
        ),
    )


def get_s3_resource():
    """Create S3 resource configured for RustFS/MinIO"""
    return boto3.resource(
        "s3",
        endpoint_url=S3_ENDPOINT,
        aws_access_key_id=S3_ACCESS_KEY,
        aws_secret_access_key=S3_SECRET_KEY,
        region_name=S3_REGION,
        config=Config(
            signature_version="s3v4",
            s3={"addressing_style": "path"},
        ),
    )


class TestBucketOperations(unittest.TestCase):
    """Test bucket-level operations"""

    @classmethod
    def setUpClass(cls):
        cls.s3 = get_s3_client()
        cls.test_bucket = f"test-bucket-{int(time.time())}"

    @classmethod
    def tearDownClass(cls):
        try:
            # Delete all objects in bucket
            response = cls.s3.list_objects_v2(Bucket=cls.test_bucket)
            for obj in response.get("Contents", []):
                cls.s3.delete_object(Bucket=cls.test_bucket, Key=obj["Key"])
            # Delete bucket
            cls.s3.delete_bucket(Bucket=cls.test_bucket)
        except ClientError:
            pass

    def test_01_create_bucket(self):
        """Test bucket creation"""
        response = self.s3.create_bucket(Bucket=self.test_bucket)
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 200)

    def test_02_list_buckets(self):
        """Test listing buckets"""
        response = self.s3.list_buckets()
        bucket_names = [b["Name"] for b in response["Buckets"]]
        self.assertIn(self.test_bucket, bucket_names)

    def test_03_head_bucket(self):
        """Test bucket existence check"""
        response = self.s3.head_bucket(Bucket=self.test_bucket)
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 200)

    def test_04_bucket_location(self):
        """Test getting bucket location"""
        response = self.s3.get_bucket_location(Bucket=self.test_bucket)
        self.assertIn("LocationConstraint", response)


class TestObjectOperations(unittest.TestCase):
    """Test object-level operations"""

    @classmethod
    def setUpClass(cls):
        cls.s3 = get_s3_client()
        cls.test_bucket = f"test-objects-{int(time.time())}"
        cls.s3.create_bucket(Bucket=cls.test_bucket)

    @classmethod
    def tearDownClass(cls):
        try:
            response = cls.s3.list_objects_v2(Bucket=cls.test_bucket)
            for obj in response.get("Contents", []):
                cls.s3.delete_object(Bucket=cls.test_bucket, Key=obj["Key"])
            cls.s3.delete_bucket(Bucket=cls.test_bucket)
        except ClientError:
            pass

    def test_01_put_object_small(self):
        """Test uploading small object (< 5MB)"""
        key = "test-small-object.txt"
        content = b"Hello, RustFS! This is a small test object."
        
        response = self.s3.put_object(
            Bucket=self.test_bucket,
            Key=key,
            Body=content,
            ContentType="text/plain",
        )
        
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 200)
        self.assertIn("ETag", response)

    def test_02_get_object(self):
        """Test downloading object"""
        key = "test-small-object.txt"
        expected_content = b"Hello, RustFS! This is a small test object."
        
        response = self.s3.get_object(Bucket=self.test_bucket, Key=key)
        content = response["Body"].read()
        
        self.assertEqual(content, expected_content)
        self.assertEqual(response["ContentType"], "text/plain")

    def test_03_head_object(self):
        """Test object metadata retrieval"""
        key = "test-small-object.txt"
        
        response = self.s3.head_object(Bucket=self.test_bucket, Key=key)
        
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 200)
        self.assertIn("ContentLength", response)
        self.assertIn("ETag", response)
        self.assertIn("LastModified", response)

    def test_04_put_object_with_metadata(self):
        """Test uploading object with custom metadata"""
        key = "test-metadata-object.txt"
        content = b"Object with metadata"
        metadata = {
            "beneficiary-id": "12345",
            "document-type": "id-card",
            "uploaded-by": "system",
        }
        
        response = self.s3.put_object(
            Bucket=self.test_bucket,
            Key=key,
            Body=content,
            Metadata=metadata,
        )
        
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 200)
        
        # Verify metadata
        head_response = self.s3.head_object(Bucket=self.test_bucket, Key=key)
        self.assertEqual(head_response["Metadata"]["beneficiary-id"], "12345")

    def test_05_copy_object(self):
        """Test copying object"""
        source_key = "test-small-object.txt"
        dest_key = "test-copied-object.txt"
        
        response = self.s3.copy_object(
            Bucket=self.test_bucket,
            Key=dest_key,
            CopySource=f"{self.test_bucket}/{source_key}",
        )
        
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 200)
        
        # Verify copy
        get_response = self.s3.get_object(Bucket=self.test_bucket, Key=dest_key)
        self.assertIsNotNone(get_response["Body"].read())

    def test_06_delete_object(self):
        """Test deleting object"""
        key = "test-to-delete.txt"
        self.s3.put_object(Bucket=self.test_bucket, Key=key, Body=b"delete me")
        
        response = self.s3.delete_object(Bucket=self.test_bucket, Key=key)
        
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 204)
        
        # Verify deletion
        with self.assertRaises(ClientError) as context:
            self.s3.head_object(Bucket=self.test_bucket, Key=key)
        self.assertEqual(context.exception.response["Error"]["Code"], "404")

    def test_07_list_objects(self):
        """Test listing objects"""
        response = self.s3.list_objects_v2(Bucket=self.test_bucket)
        
        self.assertEqual(response["ResponseMetadata"]["HTTPStatusCode"], 200)
        self.assertIn("Contents", response)
        self.assertGreater(len(response["Contents"]), 0)

    def test_08_list_objects_with_prefix(self):
        """Test listing objects with prefix filter"""
        # Create objects with different prefixes
        self.s3.put_object(Bucket=self.test_bucket, Key="documents/doc1.txt", Body=b"doc1")
        self.s3.put_object(Bucket=self.test_bucket, Key="documents/doc2.txt", Body=b"doc2")
        self.s3.put_object(Bucket=self.test_bucket, Key="images/img1.jpg", Body=b"img1")
        
        response = self.s3.list_objects_v2(Bucket=self.test_bucket, Prefix="documents/")
        
        keys = [obj["Key"] for obj in response.get("Contents", [])]
        self.assertIn("documents/doc1.txt", keys)
        self.assertIn("documents/doc2.txt", keys)
        self.assertNotIn("images/img1.jpg", keys)

    def test_09_list_objects_pagination(self):
        """Test listing objects with pagination"""
        # Create multiple objects
        for i in range(15):
            self.s3.put_object(
                Bucket=self.test_bucket,
                Key=f"pagination/obj{i:03d}.txt",
                Body=f"object {i}".encode(),
            )
        
        # List with max keys
        response = self.s3.list_objects_v2(
            Bucket=self.test_bucket,
            Prefix="pagination/",
            MaxKeys=5,
        )
        
        self.assertEqual(len(response["Contents"]), 5)
        self.assertTrue(response["IsTruncated"])
        self.assertIn("NextContinuationToken", response)
        
        # Continue listing
        response2 = self.s3.list_objects_v2(
            Bucket=self.test_bucket,
            Prefix="pagination/",
            MaxKeys=5,
            ContinuationToken=response["NextContinuationToken"],
        )
        
        self.assertEqual(len(response2["Contents"]), 5)


class TestMultipartUpload(unittest.TestCase):
    """Test multipart upload operations"""

    @classmethod
    def setUpClass(cls):
        cls.s3 = get_s3_client()
        cls.test_bucket = f"test-multipart-{int(time.time())}"
        cls.s3.create_bucket(Bucket=cls.test_bucket)

    @classmethod
    def tearDownClass(cls):
        try:
            response = cls.s3.list_objects_v2(Bucket=cls.test_bucket)
            for obj in response.get("Contents", []):
                cls.s3.delete_object(Bucket=cls.test_bucket, Key=obj["Key"])
            cls.s3.delete_bucket(Bucket=cls.test_bucket)
        except ClientError:
            pass

    def test_01_multipart_upload(self):
        """Test multipart upload for large files"""
        key = "large-file.bin"
        part_size = 5 * 1024 * 1024  # 5MB minimum part size
        num_parts = 3
        
        # Initiate multipart upload
        response = self.s3.create_multipart_upload(
            Bucket=self.test_bucket,
            Key=key,
            ContentType="application/octet-stream",
        )
        upload_id = response["UploadId"]
        
        try:
            parts = []
            for i in range(num_parts):
                part_data = os.urandom(part_size)
                part_response = self.s3.upload_part(
                    Bucket=self.test_bucket,
                    Key=key,
                    UploadId=upload_id,
                    PartNumber=i + 1,
                    Body=part_data,
                )
                parts.append({
                    "PartNumber": i + 1,
                    "ETag": part_response["ETag"],
                })
            
            # Complete multipart upload
            complete_response = self.s3.complete_multipart_upload(
                Bucket=self.test_bucket,
                Key=key,
                UploadId=upload_id,
                MultipartUpload={"Parts": parts},
            )
            
            self.assertEqual(complete_response["ResponseMetadata"]["HTTPStatusCode"], 200)
            
            # Verify file size
            head_response = self.s3.head_object(Bucket=self.test_bucket, Key=key)
            self.assertEqual(head_response["ContentLength"], part_size * num_parts)
            
        except Exception as e:
            # Abort on failure
            self.s3.abort_multipart_upload(
                Bucket=self.test_bucket,
                Key=key,
                UploadId=upload_id,
            )
            raise e

    def test_02_abort_multipart_upload(self):
        """Test aborting multipart upload"""
        key = "aborted-upload.bin"
        
        # Initiate multipart upload
        response = self.s3.create_multipart_upload(
            Bucket=self.test_bucket,
            Key=key,
        )
        upload_id = response["UploadId"]
        
        # Abort the upload
        abort_response = self.s3.abort_multipart_upload(
            Bucket=self.test_bucket,
            Key=key,
            UploadId=upload_id,
        )
        
        self.assertEqual(abort_response["ResponseMetadata"]["HTTPStatusCode"], 204)

    def test_03_list_multipart_uploads(self):
        """Test listing in-progress multipart uploads"""
        key = "pending-upload.bin"
        
        # Initiate multipart upload
        response = self.s3.create_multipart_upload(
            Bucket=self.test_bucket,
            Key=key,
        )
        upload_id = response["UploadId"]
        
        try:
            # List multipart uploads
            list_response = self.s3.list_multipart_uploads(Bucket=self.test_bucket)
            
            uploads = list_response.get("Uploads", [])
            upload_ids = [u["UploadId"] for u in uploads]
            self.assertIn(upload_id, upload_ids)
            
        finally:
            # Clean up
            self.s3.abort_multipart_upload(
                Bucket=self.test_bucket,
                Key=key,
                UploadId=upload_id,
            )


class TestPresignedUrls(unittest.TestCase):
    """Test presigned URL operations"""

    @classmethod
    def setUpClass(cls):
        cls.s3 = get_s3_client()
        cls.test_bucket = f"test-presigned-{int(time.time())}"
        cls.s3.create_bucket(Bucket=cls.test_bucket)

    @classmethod
    def tearDownClass(cls):
        try:
            response = cls.s3.list_objects_v2(Bucket=cls.test_bucket)
            for obj in response.get("Contents", []):
                cls.s3.delete_object(Bucket=cls.test_bucket, Key=obj["Key"])
            cls.s3.delete_bucket(Bucket=cls.test_bucket)
        except ClientError:
            pass

    def test_01_presigned_get_url(self):
        """Test generating presigned GET URL"""
        key = "presigned-get-test.txt"
        content = b"Content for presigned GET test"
        
        self.s3.put_object(Bucket=self.test_bucket, Key=key, Body=content)
        
        url = self.s3.generate_presigned_url(
            "get_object",
            Params={"Bucket": self.test_bucket, "Key": key},
            ExpiresIn=3600,
        )
        
        self.assertIsNotNone(url)
        self.assertIn(self.test_bucket, url)
        self.assertIn(key, url)
        self.assertIn("X-Amz-Signature", url)

    def test_02_presigned_put_url(self):
        """Test generating presigned PUT URL"""
        key = "presigned-put-test.txt"
        
        url = self.s3.generate_presigned_url(
            "put_object",
            Params={"Bucket": self.test_bucket, "Key": key},
            ExpiresIn=3600,
        )
        
        self.assertIsNotNone(url)
        self.assertIn(self.test_bucket, url)
        self.assertIn(key, url)
        self.assertIn("X-Amz-Signature", url)


class TestRangeRequests(unittest.TestCase):
    """Test range request operations"""

    @classmethod
    def setUpClass(cls):
        cls.s3 = get_s3_client()
        cls.test_bucket = f"test-range-{int(time.time())}"
        cls.s3.create_bucket(Bucket=cls.test_bucket)
        
        # Create test file
        cls.test_key = "range-test-file.bin"
        cls.test_content = b"0123456789" * 1000  # 10KB
        cls.s3.put_object(
            Bucket=cls.test_bucket,
            Key=cls.test_key,
            Body=cls.test_content,
        )

    @classmethod
    def tearDownClass(cls):
        try:
            response = cls.s3.list_objects_v2(Bucket=cls.test_bucket)
            for obj in response.get("Contents", []):
                cls.s3.delete_object(Bucket=cls.test_bucket, Key=obj["Key"])
            cls.s3.delete_bucket(Bucket=cls.test_bucket)
        except ClientError:
            pass

    def test_01_range_get_first_bytes(self):
        """Test getting first N bytes"""
        response = self.s3.get_object(
            Bucket=self.test_bucket,
            Key=self.test_key,
            Range="bytes=0-99",
        )
        
        content = response["Body"].read()
        self.assertEqual(len(content), 100)
        self.assertEqual(content, self.test_content[:100])

    def test_02_range_get_middle_bytes(self):
        """Test getting bytes from middle of file"""
        response = self.s3.get_object(
            Bucket=self.test_bucket,
            Key=self.test_key,
            Range="bytes=100-199",
        )
        
        content = response["Body"].read()
        self.assertEqual(len(content), 100)
        self.assertEqual(content, self.test_content[100:200])

    def test_03_range_get_last_bytes(self):
        """Test getting last N bytes"""
        response = self.s3.get_object(
            Bucket=self.test_bucket,
            Key=self.test_key,
            Range="bytes=-100",
        )
        
        content = response["Body"].read()
        self.assertEqual(len(content), 100)
        self.assertEqual(content, self.test_content[-100:])


class TestDataIntegrity(unittest.TestCase):
    """Test data integrity operations"""

    @classmethod
    def setUpClass(cls):
        cls.s3 = get_s3_client()
        cls.test_bucket = f"test-integrity-{int(time.time())}"
        cls.s3.create_bucket(Bucket=cls.test_bucket)

    @classmethod
    def tearDownClass(cls):
        try:
            response = cls.s3.list_objects_v2(Bucket=cls.test_bucket)
            for obj in response.get("Contents", []):
                cls.s3.delete_object(Bucket=cls.test_bucket, Key=obj["Key"])
            cls.s3.delete_bucket(Bucket=cls.test_bucket)
        except ClientError:
            pass

    def test_01_upload_download_integrity(self):
        """Test that uploaded data matches downloaded data"""
        key = "integrity-test.bin"
        original_content = os.urandom(1024 * 100)  # 100KB random data
        original_hash = hashlib.sha256(original_content).hexdigest()
        
        # Upload
        self.s3.put_object(
            Bucket=self.test_bucket,
            Key=key,
            Body=original_content,
        )
        
        # Download
        response = self.s3.get_object(Bucket=self.test_bucket, Key=key)
        downloaded_content = response["Body"].read()
        downloaded_hash = hashlib.sha256(downloaded_content).hexdigest()
        
        # Verify
        self.assertEqual(original_hash, downloaded_hash)
        self.assertEqual(len(original_content), len(downloaded_content))

    def test_02_etag_verification(self):
        """Test ETag consistency"""
        key = "etag-test.txt"
        content = b"ETag test content"
        
        # Upload
        put_response = self.s3.put_object(
            Bucket=self.test_bucket,
            Key=key,
            Body=content,
        )
        put_etag = put_response["ETag"]
        
        # Head
        head_response = self.s3.head_object(Bucket=self.test_bucket, Key=key)
        head_etag = head_response["ETag"]
        
        # Verify ETags match
        self.assertEqual(put_etag, head_etag)


class TestDeltaLakeOperations(unittest.TestCase):
    """Test Delta Lake specific operations"""

    @classmethod
    def setUpClass(cls):
        cls.s3 = get_s3_client()
        cls.test_bucket = f"test-deltalake-{int(time.time())}"
        cls.s3.create_bucket(Bucket=cls.test_bucket)

    @classmethod
    def tearDownClass(cls):
        try:
            response = cls.s3.list_objects_v2(Bucket=cls.test_bucket)
            for obj in response.get("Contents", []):
                cls.s3.delete_object(Bucket=cls.test_bucket, Key=obj["Key"])
            cls.s3.delete_bucket(Bucket=cls.test_bucket)
        except ClientError:
            pass

    def test_01_delta_log_operations(self):
        """Test Delta Lake transaction log operations"""
        # Simulate Delta Lake _delta_log structure
        table_path = "lakehouse/bronze/beneficiaries"
        
        # Create initial commit
        commit_0 = {
            "commitInfo": {
                "timestamp": int(time.time() * 1000),
                "operation": "CREATE TABLE",
            },
            "protocol": {"minReaderVersion": 1, "minWriterVersion": 2},
            "metaData": {
                "id": "test-table-id",
                "format": {"provider": "parquet"},
                "schemaString": '{"type":"struct","fields":[]}',
            },
        }
        
        self.s3.put_object(
            Bucket=self.test_bucket,
            Key=f"{table_path}/_delta_log/00000000000000000000.json",
            Body=json.dumps(commit_0).encode(),
            ContentType="application/json",
        )
        
        # Verify commit log
        response = self.s3.get_object(
            Bucket=self.test_bucket,
            Key=f"{table_path}/_delta_log/00000000000000000000.json",
        )
        
        content = json.loads(response["Body"].read().decode())
        self.assertIn("commitInfo", content)
        self.assertIn("protocol", content)

    def test_02_parquet_file_operations(self):
        """Test Parquet file operations (simulated)"""
        table_path = "lakehouse/bronze/beneficiaries"
        
        # Simulate Parquet file (just binary data for testing)
        parquet_data = os.urandom(1024)
        
        self.s3.put_object(
            Bucket=self.test_bucket,
            Key=f"{table_path}/part-00000-abc123.parquet",
            Body=parquet_data,
            ContentType="application/octet-stream",
        )
        
        # Verify file exists and is readable
        response = self.s3.head_object(
            Bucket=self.test_bucket,
            Key=f"{table_path}/part-00000-abc123.parquet",
        )
        
        self.assertEqual(response["ContentLength"], 1024)


def run_tests():
    """Run all regression tests"""
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    
    # Add test classes
    suite.addTests(loader.loadTestsFromTestCase(TestBucketOperations))
    suite.addTests(loader.loadTestsFromTestCase(TestObjectOperations))
    suite.addTests(loader.loadTestsFromTestCase(TestMultipartUpload))
    suite.addTests(loader.loadTestsFromTestCase(TestPresignedUrls))
    suite.addTests(loader.loadTestsFromTestCase(TestRangeRequests))
    suite.addTests(loader.loadTestsFromTestCase(TestDataIntegrity))
    suite.addTests(loader.loadTestsFromTestCase(TestDeltaLakeOperations))
    
    # Run tests
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)
    
    # Return exit code
    return 0 if result.wasSuccessful() else 1


if __name__ == "__main__":
    sys.exit(run_tests())
