CREATE TABLE `tenant_usage` (
	`id` int AUTO_INCREMENT NOT NULL,
	`tenant_id` int NOT NULL,
	`metric_type` varchar(100) NOT NULL,
	`metric_value` float NOT NULL,
	`period` varchar(20) NOT NULL,
	`recorded_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `tenant_usage_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `tenant_users` (
	`id` int AUTO_INCREMENT NOT NULL,
	`tenant_id` int NOT NULL,
	`user_id` int NOT NULL,
	`role` enum('owner','admin','member') NOT NULL DEFAULT 'member',
	`joined_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `tenant_users_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `tenants` (
	`id` int AUTO_INCREMENT NOT NULL,
	`tenant_code` varchar(50) NOT NULL,
	`tenant_name` varchar(255) NOT NULL,
	`status` enum('active','suspended','inactive') NOT NULL DEFAULT 'active',
	`configuration` text,
	`kafka_topic_prefix` varchar(50) NOT NULL,
	`redis_namespace` varchar(50) NOT NULL,
	`permify_tenant_id` varchar(100),
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `tenants_id` PRIMARY KEY(`id`),
	CONSTRAINT `tenants_tenant_code_unique` UNIQUE(`tenant_code`)
);
