CREATE TABLE `request_artifact` (
	`id` text PRIMARY KEY NOT NULL,
	`requestId` text NOT NULL,
	`variant` text NOT NULL,
	`name` text NOT NULL,
	`path` text NOT NULL,
	`sizeBytes` integer NOT NULL,
	`uploadedAt` integer NOT NULL,
	FOREIGN KEY (`requestId`) REFERENCES `request`(`id`) ON UPDATE no action ON DELETE cascade
);
--> statement-breakpoint
CREATE UNIQUE INDEX `request_artifact_unique` ON `request_artifact` (`requestId`,`variant`,`name`);