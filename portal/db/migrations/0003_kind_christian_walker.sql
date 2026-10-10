CREATE TABLE `device_link` (
	`id` text PRIMARY KEY NOT NULL,
	`userCode` text NOT NULL,
	`pollTokenHash` text NOT NULL,
	`deviceName` text NOT NULL,
	`status` text DEFAULT 'pending' NOT NULL,
	`userId` text,
	`createdAt` integer NOT NULL,
	`expiresAt` integer NOT NULL,
	FOREIGN KEY (`userId`) REFERENCES `user`(`id`) ON UPDATE no action ON DELETE cascade
);
--> statement-breakpoint
CREATE UNIQUE INDEX `device_link_userCode_unique` ON `device_link` (`userCode`);--> statement-breakpoint
CREATE TABLE `device` (
	`id` text PRIMARY KEY NOT NULL,
	`userId` text NOT NULL,
	`name` text NOT NULL,
	`tokenHash` text NOT NULL,
	`createdAt` integer NOT NULL,
	`lastSeenAt` integer,
	`revokedAt` integer,
	FOREIGN KEY (`userId`) REFERENCES `user`(`id`) ON UPDATE no action ON DELETE cascade
);
--> statement-breakpoint
CREATE UNIQUE INDEX `device_tokenHash_unique` ON `device` (`tokenHash`);--> statement-breakpoint
CREATE TABLE `event` (
	`id` text PRIMARY KEY NOT NULL,
	`at` integer NOT NULL,
	`requestId` text,
	`workerId` text,
	`subjectUserId` text,
	`actor` text NOT NULL,
	`type` text NOT NULL,
	`detail` text,
	FOREIGN KEY (`requestId`) REFERENCES `request`(`id`) ON UPDATE no action ON DELETE set null,
	FOREIGN KEY (`workerId`) REFERENCES `worker`(`id`) ON UPDATE no action ON DELETE set null,
	FOREIGN KEY (`subjectUserId`) REFERENCES `user`(`id`) ON UPDATE no action ON DELETE set null
);
--> statement-breakpoint
CREATE INDEX `event_request_at` ON `event` (`requestId`,`at`);--> statement-breakpoint
CREATE INDEX `event_at` ON `event` (`at`);--> statement-breakpoint
CREATE TABLE `user_cloud` (
	`userId` text PRIMARY KEY NOT NULL,
	`access` text NOT NULL,
	`maxActive` integer,
	`dailySeconds` integer,
	`note` text,
	`updatedAt` integer NOT NULL,
	FOREIGN KEY (`userId`) REFERENCES `user`(`id`) ON UPDATE no action ON DELETE cascade
);
--> statement-breakpoint
CREATE TABLE `worker` (
	`id` text PRIMARY KEY NOT NULL,
	`name` text NOT NULL,
	`tokenHash` text NOT NULL,
	`createdAt` integer NOT NULL,
	`revokedAt` integer,
	`paused` integer DEFAULT false NOT NULL,
	`pausedBy` text,
	`pauseReason` text,
	`lastSeenAt` integer,
	`state` text,
	`blockedCode` text,
	`blockedDetail` text,
	`health` text,
	`consecutiveFailures` integer DEFAULT 0 NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX `worker_tokenHash_unique` ON `worker` (`tokenHash`);--> statement-breakpoint
ALTER TABLE `request_artifact` ADD `kind` text DEFAULT 'video' NOT NULL;--> statement-breakpoint
ALTER TABLE `request_artifact` ADD `sha256` text;--> statement-breakpoint
ALTER TABLE `request_artifact` ADD `status` text DEFAULT 'ready' NOT NULL;--> statement-breakpoint
ALTER TABLE `request_artifact` ADD `partSize` integer;--> statement-breakpoint
ALTER TABLE `request_artifact` ADD `receivedParts` text;--> statement-breakpoint
ALTER TABLE `request_artifact` ADD `receivedAt` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `kind` text DEFAULT 'manual' NOT NULL;--> statement-breakpoint
ALTER TABLE `request` ADD `title` text;--> statement-breakpoint
ALTER TABLE `request` ADD `spec` text;--> statement-breakpoint
ALTER TABLE `request` ADD `targetSteamId` text;--> statement-breakpoint
ALTER TABLE `request` ADD `demoSizeBytes` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `deviceId` text REFERENCES device(id) ON DELETE set null;--> statement-breakpoint
ALTER TABLE `request` ADD `estCaptureSeconds` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `estimatedSeconds` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `enqueuedAt` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `priorityBoost` integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE `request` ADD `workerId` text REFERENCES worker(id) ON DELETE set null;--> statement-breakpoint
ALTER TABLE `request` ADD `leaseExpiresAt` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `attempt` integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE `request` ADD `maxAttempts` integer DEFAULT 2 NOT NULL;--> statement-breakpoint
ALTER TABLE `request` ADD `stage` text;--> statement-breakpoint
ALTER TABLE `request` ADD `progressPercent` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `progressDetail` text;--> statement-breakpoint
ALTER TABLE `request` ADD `finishedAt` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `machineSeconds` integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE `request` ADD `failureCode` text;--> statement-breakpoint
ALTER TABLE `request` ADD `failureDetail` text;--> statement-breakpoint
ALTER TABLE `request` ADD `cancelRequestedAt` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `canceledBy` text;--> statement-breakpoint
CREATE INDEX `request_status_enqueued` ON `request` (`status`,`enqueuedAt`);--> statement-breakpoint
CREATE INDEX `request_user_created` ON `request` (`userId`,`createdAt`);--> statement-breakpoint
CREATE INDEX `request_worker_status` ON `request` (`workerId`,`status`);