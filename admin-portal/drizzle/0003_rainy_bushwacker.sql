CREATE TABLE `notification_preferences` (
	`id` int AUTO_INCREMENT NOT NULL,
	`userId` int NOT NULL,
	`notifyOnRoleChange` boolean NOT NULL DEFAULT true,
	`notifyOnBatchOperations` boolean NOT NULL DEFAULT true,
	`notifyOnProgramChanges` boolean NOT NULL DEFAULT false,
	`notifyOnMccRuleChanges` boolean NOT NULL DEFAULT false,
	`emailAddress` varchar(320),
	`createdAt` timestamp NOT NULL DEFAULT (now()),
	`updatedAt` timestamp NOT NULL DEFAULT (now()) ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT `notification_preferences_id` PRIMARY KEY(`id`)
);
