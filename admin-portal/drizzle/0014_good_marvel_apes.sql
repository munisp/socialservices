CREATE TABLE `activity_executions` (
	`id` int AUTO_INCREMENT NOT NULL,
	`workflow_id` varchar(255) NOT NULL,
	`activity_id` varchar(255) NOT NULL,
	`activity_type` varchar(100) NOT NULL,
	`status` enum('scheduled','started','completed','failed','timeout','cancelled') NOT NULL,
	`input` json,
	`result` json,
	`error` text,
	`attempt` int NOT NULL DEFAULT 1,
	`start_time` timestamp NOT NULL,
	`end_time` timestamp,
	`duration` int,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `activity_executions_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `workflow_alerts` (
	`id` int AUTO_INCREMENT NOT NULL,
	`workflow_id` varchar(255) NOT NULL,
	`alert_type` enum('timeout','high_failure_rate','slow_execution','stuck_workflow','resource_exhaustion') NOT NULL,
	`severity` enum('low','medium','high','critical') NOT NULL,
	`message` text NOT NULL,
	`resolved` int NOT NULL DEFAULT 0,
	`resolved_at` timestamp,
	`resolved_by` varchar(255),
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `workflow_alerts_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `workflow_executions` (
	`id` int AUTO_INCREMENT NOT NULL,
	`workflow_id` varchar(255) NOT NULL,
	`workflow_type` varchar(100) NOT NULL,
	`run_id` varchar(255) NOT NULL,
	`status` enum('running','completed','failed','cancelled','timeout') NOT NULL,
	`input` json,
	`result` json,
	`error` text,
	`start_time` timestamp NOT NULL,
	`end_time` timestamp,
	`duration` int,
	`initiated_by` varchar(255),
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `workflow_executions_id` PRIMARY KEY(`id`),
	CONSTRAINT `workflow_executions_workflow_id_unique` UNIQUE(`workflow_id`)
);
--> statement-breakpoint
CREATE TABLE `workflow_metrics` (
	`id` int AUTO_INCREMENT NOT NULL,
	`workflow_type` varchar(100) NOT NULL,
	`date` timestamp NOT NULL,
	`total_executions` int NOT NULL DEFAULT 0,
	`successful_executions` int NOT NULL DEFAULT 0,
	`failed_executions` int NOT NULL DEFAULT 0,
	`average_duration` int,
	`min_duration` int,
	`max_duration` int,
	`p50_duration` int,
	`p95_duration` int,
	`p99_duration` int,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`updated_at` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `workflow_metrics_id` PRIMARY KEY(`id`)
);
