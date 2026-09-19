CREATE TABLE `saved_search_filters` (
	`id` int AUTO_INCREMENT NOT NULL,
	`user_id` int NOT NULL,
	`name` varchar(255) NOT NULL,
	`query` text NOT NULL,
	`entity_type` varchar(50) NOT NULL,
	`created_at` timestamp NOT NULL DEFAULT (now()),
	CONSTRAINT `saved_search_filters_id` PRIMARY KEY(`id`)
);
