-- Migration: Add soft delete fields to benefit_programs table
-- This enables soft delete functionality where programs are marked as deleted
-- rather than being permanently removed from the database

ALTER TABLE `benefit_programs` 
ADD COLUMN `deletedAt` timestamp NULL DEFAULT NULL,
ADD COLUMN `deletedBy` int NULL DEFAULT NULL;

-- Add foreign key constraint for deletedBy
ALTER TABLE `benefit_programs`
ADD CONSTRAINT `benefit_programs_deletedBy_users_id_fk` 
FOREIGN KEY (`deletedBy`) REFERENCES `users`(`id`) ON DELETE SET NULL ON UPDATE CASCADE;

-- Add index for efficient filtering of non-deleted programs
CREATE INDEX `idx_benefit_programs_deleted_at` ON `benefit_programs`(`deletedAt`);

-- Add composite index for listing active (non-deleted) programs
CREATE INDEX `idx_benefit_programs_status_deleted` ON `benefit_programs`(`status`, `deletedAt`);
