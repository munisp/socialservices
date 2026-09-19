CREATE TABLE `approval_requests` (
	`id` int AUTO_INCREMENT NOT NULL,
	`request_type` enum('ROLE_CHANGE','BATCH_PROGRAM_UPDATE','BATCH_FLAG_TOGGLE','PROGRAM_DELETE','USER_DELETE') NOT NULL,
	`requested_by` int NOT NULL,
	`target_id` int,
	`request_data` text NOT NULL,
	`justification` text NOT NULL,
	`status` enum('pending','approved','rejected') NOT NULL DEFAULT 'pending',
	`required_approvals` int NOT NULL DEFAULT 2,
	`current_approvals` int NOT NULL DEFAULT 0,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	`completed_at` timestamp,
	CONSTRAINT `approval_requests_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
CREATE TABLE `approvals` (
	`id` int AUTO_INCREMENT NOT NULL,
	`request_id` int NOT NULL,
	`approved_by` int NOT NULL,
	`decision` enum('approved','rejected') NOT NULL,
	`comment` text,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `approvals_id` PRIMARY KEY(`id`)
);
