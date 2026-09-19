CREATE TABLE `report_history` (
	`id` int AUTO_INCREMENT NOT NULL,
	`report_id` int NOT NULL,
	`generated_at` timestamp NOT NULL DEFAULT (now()),
	`report_data` text NOT NULL,
	`status` enum('success','failed') NOT NULL,
	`error_message` text,
	CONSTRAINT `report_history_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `scheduled_reports` (
	`id` int AUTO_INCREMENT NOT NULL,
	`name` varchar(255) NOT NULL,
	`description` text,
	`report_type` enum('weekly','monthly') NOT NULL,
	`recipients` text NOT NULL,
	`is_active` int NOT NULL DEFAULT 1,
	`last_run_at` timestamp,
	`next_run_at` timestamp NOT NULL,
	`created_by` int,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `scheduled_reports_id` PRIMARY KEY(`id`)
);
