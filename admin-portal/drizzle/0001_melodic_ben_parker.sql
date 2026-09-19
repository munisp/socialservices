CREATE TABLE `benefit_programs` (
	`id` int AUTO_INCREMENT NOT NULL,
	`name` varchar(255) NOT NULL,
	`accountType` varchar(100) NOT NULL,
	`description` text,
	`status` enum('active','inactive','pending_review') NOT NULL DEFAULT 'active',
	`createdAt` timestamp NOT NULL DEFAULT (now()),
	`updatedAt` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	`createdBy` int,
	CONSTRAINT `benefit_programs_id` PRIMARY KEY(`id`),
	CONSTRAINT `benefit_programs_accountType_unique` UNIQUE(`accountType`)
);
--> statement-breakpoint
CREATE TABLE `disbursement_schedules` (
	`id` int AUTO_INCREMENT NOT NULL,
	`programId` int NOT NULL,
	`scheduledDate` timestamp NOT NULL,
	`amount` int NOT NULL,
	`beneficiaryCount` int,
	`status` enum('pending','processing','completed','failed') NOT NULL DEFAULT 'pending',
	`metadata` json,
	`createdAt` timestamp NOT NULL DEFAULT (now()),
	`updatedAt` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	`createdBy` int,
	CONSTRAINT `disbursement_schedules_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `feature_flags` (
	`id` int AUTO_INCREMENT NOT NULL,
	`name` varchar(100) NOT NULL,
	`description` text,
	`enabled` boolean NOT NULL DEFAULT false,
	`environment` enum('all','production','staging','development') NOT NULL DEFAULT 'all',
	`createdAt` timestamp NOT NULL DEFAULT (now()),
	`updatedAt` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	`updatedBy` int,
	CONSTRAINT `feature_flags_id` PRIMARY KEY(`id`),
	CONSTRAINT `feature_flags_name_unique` UNIQUE(`name`)
);
--> statement-breakpoint
CREATE TABLE `mcc_database` (
	`id` int AUTO_INCREMENT NOT NULL,
	`mccCode` varchar(10) NOT NULL,
	`description` varchar(255) NOT NULL,
	`category` varchar(100),
	CONSTRAINT `mcc_database_id` PRIMARY KEY(`id`),
	CONSTRAINT `mcc_database_mccCode_unique` UNIQUE(`mccCode`)
);
--> statement-breakpoint
CREATE TABLE `mcc_rule_audit` (
	`id` int AUTO_INCREMENT NOT NULL,
	`programId` int NOT NULL,
	`action` enum('add_mcc','remove_mcc','update_program') NOT NULL,
	`mccCode` varchar(10),
	`justification` text NOT NULL,
	`performedBy` int NOT NULL,
	`performedAt` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `mcc_rule_audit_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `mcc_rules` (
	`id` int AUTO_INCREMENT NOT NULL,
	`programId` int NOT NULL,
	`mccCode` varchar(10) NOT NULL,
	`mccDescription` varchar(255),
	`createdAt` timestamp NOT NULL DEFAULT (now()),
	`createdBy` int,
	CONSTRAINT `mcc_rules_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
ALTER TABLE `benefit_programs` ADD CONSTRAINT `benefit_programs_createdBy_users_id_fk` FOREIGN KEY (`createdBy`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `disbursement_schedules` ADD CONSTRAINT `disbursement_schedules_programId_benefit_programs_id_fk` FOREIGN KEY (`programId`) REFERENCES `benefit_programs`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `disbursement_schedules` ADD CONSTRAINT `disbursement_schedules_createdBy_users_id_fk` FOREIGN KEY (`createdBy`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `feature_flags` ADD CONSTRAINT `feature_flags_updatedBy_users_id_fk` FOREIGN KEY (`updatedBy`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `mcc_rule_audit` ADD CONSTRAINT `mcc_rule_audit_programId_benefit_programs_id_fk` FOREIGN KEY (`programId`) REFERENCES `benefit_programs`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `mcc_rule_audit` ADD CONSTRAINT `mcc_rule_audit_performedBy_users_id_fk` FOREIGN KEY (`performedBy`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `mcc_rules` ADD CONSTRAINT `mcc_rules_programId_benefit_programs_id_fk` FOREIGN KEY (`programId`) REFERENCES `benefit_programs`(`id`) ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `mcc_rules` ADD CONSTRAINT `mcc_rules_createdBy_users_id_fk` FOREIGN KEY (`createdBy`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;