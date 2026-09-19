CREATE TABLE `middleware_alerts` (
	`id` int AUTO_INCREMENT NOT NULL,
	`component` varchar(50) NOT NULL,
	`alert_type` varchar(50) NOT NULL,
	`severity` enum('info','warning','critical') NOT NULL,
	`message` text NOT NULL,
	`details` text,
	`status` enum('active','acknowledged','resolved') NOT NULL DEFAULT 'active',
	`triggered_at` timestamp NOT NULL DEFAULT (now()),
	`acknowledged_at` timestamp,
	`acknowledged_by` int,
	`resolved_at` timestamp,
	`resolved_by` int,
	CONSTRAINT `middleware_alerts_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `middleware_health_checks` (
	`id` int AUTO_INCREMENT NOT NULL,
	`component` varchar(50) NOT NULL,
	`status` enum('healthy','degraded','unhealthy') NOT NULL,
	`response_time` int,
	`error_message` text,
	`checked_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `middleware_health_checks_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `middleware_metrics` (
	`id` int AUTO_INCREMENT NOT NULL,
	`component` varchar(50) NOT NULL,
	`metric_type` varchar(100) NOT NULL,
	`metric_value` float NOT NULL,
	`unit` varchar(20),
	`timestamp` timestamp NOT NULL DEFAULT (now()),
	`metadata` text,
	CONSTRAINT `middleware_metrics_id` PRIMARY KEY(`id`)
);
