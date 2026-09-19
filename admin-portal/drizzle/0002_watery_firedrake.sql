CREATE TABLE `admin_action_audit` (
	`id` int AUTO_INCREMENT NOT NULL,
	`action` varchar(100) NOT NULL,
	`targetUserId` int,
	`details` json,
	`justification` text,
	`performedBy` int NOT NULL,
	`performedAt` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `admin_action_audit_id` PRIMARY KEY(`id`)
);
--> statement-breakpoint
ALTER TABLE `admin_action_audit` ADD CONSTRAINT `admin_action_audit_targetUserId_users_id_fk` FOREIGN KEY (`targetUserId`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;--> statement-breakpoint
ALTER TABLE `admin_action_audit` ADD CONSTRAINT `admin_action_audit_performedBy_users_id_fk` FOREIGN KEY (`performedBy`) REFERENCES `users`(`id`) ON DELETE no action ON UPDATE no action;