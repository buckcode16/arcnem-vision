DROP INDEX "presigned_uploads_object_key_device_uidx";--> statement-breakpoint
ALTER TABLE "documents" ALTER COLUMN "device_id" DROP NOT NULL;--> statement-breakpoint
ALTER TABLE "presigned_uploads" ALTER COLUMN "device_id" DROP NOT NULL;--> statement-breakpoint
ALTER TABLE "presigned_uploads" ADD COLUMN "organization_id" uuid;--> statement-breakpoint
ALTER TABLE "presigned_uploads" ADD COLUMN "project_id" uuid;--> statement-breakpoint
UPDATE "presigned_uploads" AS pu
SET
	"organization_id" = d."organization_id",
	"project_id" = d."project_id"
FROM "devices" AS d
WHERE pu."device_id" = d."id";--> statement-breakpoint
ALTER TABLE "presigned_uploads" ALTER COLUMN "organization_id" SET NOT NULL;--> statement-breakpoint
ALTER TABLE "presigned_uploads" ALTER COLUMN "project_id" SET NOT NULL;--> statement-breakpoint
ALTER TABLE "presigned_uploads" ADD CONSTRAINT "presigned_uploads_organization_id_organizations_id_fk" FOREIGN KEY ("organization_id") REFERENCES "public"."organizations"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "presigned_uploads" ADD CONSTRAINT "presigned_uploads_project_id_projects_id_fk" FOREIGN KEY ("project_id") REFERENCES "public"."projects"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "presigned_uploads_object_key_uidx" ON "presigned_uploads" USING btree ("object_key");--> statement-breakpoint
CREATE INDEX "presigned_uploads_organization_id_idx" ON "presigned_uploads" USING btree ("organization_id");--> statement-breakpoint
CREATE INDEX "presigned_uploads_project_id_idx" ON "presigned_uploads" USING btree ("project_id");
