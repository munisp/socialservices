CREATE TABLE `event_replay_logs` (
	`id` int AUTO_INCREMENT NOT NULL,
	`replay_id` int NOT NULL,
	`event_offset` varchar(100) NOT NULL,
	`event_type` varchar(100) NOT NULL,
	`event_data` text,
	`processed_at` timestamp NOT NULL DEFAULT (now()),
	`success` boolean NOT NULL,
	`error_message` text,
	CONSTRAINT `event_replay_logs_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `event_replays` (
	`id` int AUTO_INCREMENT NOT NULL,
	`replay_name` varchar(255) NOT NULL,
	`snapshot_id` int,
	`topics` text NOT NULL,
	`start_offset` varchar(100),
	`end_offset` varchar(100),
	`event_filter` text,
	`status` enum('pending','running','completed','failed','cancelled') NOT NULL DEFAULT 'pending',
	`progress` int NOT NULL DEFAULT 0,
	`events_processed` int NOT NULL DEFAULT 0,
	`events_total` int NOT NULL DEFAULT 0,
	`error_message` text,
	`started_by` int NOT NULL,
	`started_at` timestamp NOT NULL DEFAULT (now()),
	`completed_at` timestamp,
	CONSTRAINT `event_replays_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `event_snapshots` (
	`id` int AUTO_INCREMENT NOT NULL,
	`snapshot_name` varchar(255) NOT NULL,
	`description` text,
	`event_count` int NOT NULL,
	`start_offset` varchar(100) NOT NULL,
	`end_offset` varchar(100) NOT NULL,
	`topics` text NOT NULL,
	`created_by` int NOT NULL,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `event_snapshots_id` PRIMARY KEY(`id`)
);
