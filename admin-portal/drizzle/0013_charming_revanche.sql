CREATE TABLE `ml_ab_tests` (
	`id` int AUTO_INCREMENT NOT NULL,
	`test_name` varchar(100) NOT NULL,
	`model_a_id` int NOT NULL,
	`model_b_id` int NOT NULL,
	`traffic_split` int NOT NULL DEFAULT 50,
	`status` enum('running','completed','cancelled') NOT NULL DEFAULT 'running',
	`started_at` timestamp NOT NULL DEFAULT (now()),
	`ended_at` timestamp,
	`results` text,
	CONSTRAINT `ml_ab_tests_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `ml_features` (
	`id` int AUTO_INCREMENT NOT NULL,
	`feature_name` varchar(100) NOT NULL,
	`feature_type` varchar(50) NOT NULL,
	`description` text,
	`importance` float,
	`model_type` varchar(50) NOT NULL,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `ml_features_id` PRIMARY KEY(`id`),
	CONSTRAINT `ml_features_feature_name_unique` UNIQUE(`feature_name`)
);
--> statement-breakpoint
CREATE TABLE `ml_models` (
	`id` int AUTO_INCREMENT NOT NULL,
	`model_name` varchar(100) NOT NULL,
	`model_type` varchar(50) NOT NULL,
	`version` varchar(20) NOT NULL,
	`algorithm` varchar(50) NOT NULL,
	`hyperparameters` text,
	`metrics` text,
	`status` enum('training','active','archived') NOT NULL DEFAULT 'training',
	`trained_at` timestamp NOT NULL DEFAULT (now()),
	`created_by` int NOT NULL,
	CONSTRAINT `ml_models_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `ml_training_jobs` (
	`id` int AUTO_INCREMENT NOT NULL,
	`model_id` int NOT NULL,
	`job_type` varchar(50) NOT NULL,
	`dataset_size` int NOT NULL,
	`training_duration` int,
	`status` enum('pending','running','completed','failed') NOT NULL DEFAULT 'pending',
	`error_message` text,
	`started_at` timestamp,
	`completed_at` timestamp,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `ml_training_jobs_id` PRIMARY KEY(`id`)
);
