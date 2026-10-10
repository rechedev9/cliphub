ALTER TABLE `device_link` ADD `startIpHash` text;--> statement-breakpoint
ALTER TABLE `request` ADD `uploadingAt` integer;--> statement-breakpoint
ALTER TABLE `request` ADD `machineRequeues` integer DEFAULT 0 NOT NULL;