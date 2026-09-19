CREATE TABLE `blockchain_audit_trail` (
	`id` int AUTO_INCREMENT NOT NULL,
	`entity_type` varchar(50) NOT NULL,
	`entity_id` int NOT NULL,
	`action` varchar(50) NOT NULL,
	`previous_hash` varchar(255),
	`current_hash` varchar(255) NOT NULL,
	`blockchain_tx_id` int,
	`created_by` int NOT NULL,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `blockchain_audit_trail_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `blockchain_identities` (
	`id` int AUTO_INCREMENT NOT NULL,
	`user_id` int NOT NULL,
	`did` varchar(255) NOT NULL,
	`public_key` text NOT NULL,
	`verification_method` text,
	`status` enum('active','revoked') NOT NULL DEFAULT 'active',
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `blockchain_identities_id` PRIMARY KEY(`id`),
	CONSTRAINT `blockchain_identities_user_id_unique` UNIQUE(`user_id`),
	CONSTRAINT `blockchain_identities_did_unique` UNIQUE(`did`)
);
--> statement-breakpoint
CREATE TABLE `blockchain_transactions` (
	`id` int AUTO_INCREMENT NOT NULL,
	`transaction_id` varchar(100) NOT NULL,
	`block_hash` varchar(255) NOT NULL,
	`block_number` int NOT NULL,
	`transaction_hash` varchar(255) NOT NULL,
	`chaincode_name` varchar(100) NOT NULL,
	`function_name` varchar(100) NOT NULL,
	`payload` text NOT NULL,
	`status` enum('pending','confirmed','failed') NOT NULL DEFAULT 'pending',
	`timestamp` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `blockchain_transactions_id` PRIMARY KEY(`id`),
	CONSTRAINT `blockchain_transactions_transaction_id_unique` UNIQUE(`transaction_id`),
	CONSTRAINT `blockchain_transactions_transaction_hash_unique` UNIQUE(`transaction_hash`)
);
