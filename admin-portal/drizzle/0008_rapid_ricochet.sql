CREATE TABLE `beneficiaries` (
	`id` int AUTO_INCREMENT NOT NULL,
	`first_name` varchar(100) NOT NULL,
	`last_name` varchar(100) NOT NULL,
	`date_of_birth` timestamp NOT NULL,
	`national_id` varchar(50),
	`phone_number` varchar(20),
	`email` varchar(320),
	`address` text,
	`city` varchar(100),
	`state` varchar(100),
	`postal_code` varchar(20),
	`enrollment_status` enum('pending','approved','rejected','suspended') NOT NULL DEFAULT 'pending',
	`kyc_status` enum('not_started','in_progress','completed','failed') NOT NULL DEFAULT 'not_started',
	`enrolled_at` timestamp NOT NULL DEFAULT (now()),
	`approved_at` timestamp,
	`approved_by` int,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `beneficiaries_id` PRIMARY KEY(`id`),
	CONSTRAINT `beneficiaries_national_id_unique` UNIQUE(`national_id`)
);
--> statement-breakpoint
CREATE TABLE `benefit_cards` (
	`id` int AUTO_INCREMENT NOT NULL,
	`beneficiary_id` int NOT NULL,
	`card_number` varchar(20) NOT NULL,
	`card_type` enum('physical','virtual') NOT NULL,
	`status` enum('pending','active','blocked','expired','lost','stolen') NOT NULL DEFAULT 'pending',
	`issued_at` timestamp NOT NULL DEFAULT (now()),
	`expires_at` timestamp NOT NULL,
	`activated_at` timestamp,
	`blocked_at` timestamp,
	`block_reason` text,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `benefit_cards_id` PRIMARY KEY(`id`),
	CONSTRAINT `benefit_cards_card_number_unique` UNIQUE(`card_number`)
);
--> statement-breakpoint
CREATE TABLE `fraud_alerts` (
	`id` int AUTO_INCREMENT NOT NULL,
	`transaction_id` int,
	`beneficiary_id` int NOT NULL,
	`alert_type` enum('mcc_violation','unusual_spending','velocity_check','duplicate_transaction','suspicious_merchant','other') NOT NULL,
	`severity` enum('low','medium','high','critical') NOT NULL,
	`description` text NOT NULL,
	`status` enum('open','investigating','resolved','false_positive') NOT NULL DEFAULT 'open',
	`assigned_to` int,
	`resolved_by` int,
	`resolved_at` timestamp,
	`resolution` text,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `fraud_alerts_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `kyc_documents` (
	`id` int AUTO_INCREMENT NOT NULL,
	`beneficiary_id` int NOT NULL,
	`document_type` enum('national_id','passport','drivers_license','birth_certificate','proof_of_address','photo','other') NOT NULL,
	`document_url` varchar(500) NOT NULL,
	`file_key` varchar(500) NOT NULL,
	`file_name` varchar(255) NOT NULL,
	`mime_type` varchar(100),
	`file_size` int,
	`verification_status` enum('pending','verified','rejected') NOT NULL DEFAULT 'pending',
	`verified_by` int,
	`verified_at` timestamp,
	`rejection_reason` text,
	`uploaded_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `kyc_documents_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `program_enrollments` (
	`id` int AUTO_INCREMENT NOT NULL,
	`beneficiary_id` int NOT NULL,
	`program_id` int NOT NULL,
	`enrollment_date` timestamp NOT NULL DEFAULT (now()),
	`status` enum('active','inactive','suspended') NOT NULL DEFAULT 'active',
	`monthly_allocation` int NOT NULL,
	`last_disbursement` timestamp,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `program_enrollments_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `transactions` (
	`id` int AUTO_INCREMENT NOT NULL,
	`transaction_id` varchar(100) NOT NULL,
	`beneficiary_id` int NOT NULL,
	`card_id` int,
	`program_id` int NOT NULL,
	`merchant_name` varchar(255),
	`merchant_id` varchar(100),
	`mcc_code` varchar(10) NOT NULL,
	`mcc_description` varchar(255),
	`amount` int NOT NULL,
	`currency` varchar(3) NOT NULL DEFAULT 'NGN',
	`transaction_type` enum('purchase','refund','reversal') NOT NULL,
	`status` enum('pending','approved','declined','reversed') NOT NULL,
	`compliance_status` enum('compliant','non_compliant','under_review') NOT NULL DEFAULT 'compliant',
	`decline_reason` text,
	`fraud_score` int,
	`fraud_flags` json,
	`transaction_date` timestamp NOT NULL,
	`settled_at` timestamp,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `transactions_id` PRIMARY KEY(`id`),
	CONSTRAINT `transactions_transaction_id_unique` UNIQUE(`transaction_id`)
);
--> statement-breakpoint
ALTER TABLE `beneficiaries` ADD CONSTRAINT `beneficiaries_approved_by_users_id_fk` FOREIGN KEY (`approved_by`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `benefit_cards` ADD CONSTRAINT `benefit_cards_beneficiary_id_beneficiaries_id_fk` FOREIGN KEY (`beneficiary_id`) REFERENCES `beneficiaries`(`id`) ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `fraud_alerts` ADD CONSTRAINT `fraud_alerts_transaction_id_transactions_id_fk` FOREIGN KEY (`transaction_id`) REFERENCES `transactions`(`id`) ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `fraud_alerts` ADD CONSTRAINT `fraud_alerts_beneficiary_id_beneficiaries_id_fk` FOREIGN KEY (`beneficiary_id`) REFERENCES `beneficiaries`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `fraud_alerts` ADD CONSTRAINT `fraud_alerts_assigned_to_users_id_fk` FOREIGN KEY (`assigned_to`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `fraud_alerts` ADD CONSTRAINT `fraud_alerts_resolved_by_users_id_fk` FOREIGN KEY (`resolved_by`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `kyc_documents` ADD CONSTRAINT `kyc_documents_beneficiary_id_beneficiaries_id_fk` FOREIGN KEY (`beneficiary_id`) REFERENCES `beneficiaries`(`id`) ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `kyc_documents` ADD CONSTRAINT `kyc_documents_verified_by_users_id_fk` FOREIGN KEY (`verified_by`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `program_enrollments` ADD CONSTRAINT `program_enrollments_beneficiary_id_beneficiaries_id_fk` FOREIGN KEY (`beneficiary_id`) REFERENCES `beneficiaries`(`id`) ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `program_enrollments` ADD CONSTRAINT `program_enrollments_program_id_benefit_programs_id_fk` FOREIGN KEY (`program_id`) REFERENCES `benefit_programs`(`id`) ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `transactions` ADD CONSTRAINT `transactions_beneficiary_id_beneficiaries_id_fk` FOREIGN KEY (`beneficiary_id`) REFERENCES `beneficiaries`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `transactions` ADD CONSTRAINT `transactions_card_id_benefit_cards_id_fk` FOREIGN KEY (`card_id`) REFERENCES `benefit_cards`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `transactions` ADD CONSTRAINT `transactions_program_id_benefit_programs_id_fk` FOREIGN KEY (`program_id`) REFERENCES `benefit_programs`(`id`) ON DELETE no action ON UPDATE no action;