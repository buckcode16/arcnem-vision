CREATE TABLE "document_segmentations" (
	"id" uuid PRIMARY KEY DEFAULT uuidv7() NOT NULL,
	"source_document_id" uuid NOT NULL,
	"segmented_document_id" uuid,
	"model_id" uuid NOT NULL,
	"input" jsonb DEFAULT '{}' NOT NULL,
	"result" jsonb NOT NULL,
	"created_at" timestamp DEFAULT now() NOT NULL,
	"updated_at" timestamp DEFAULT now() NOT NULL
);
--> statement-breakpoint
DROP INDEX "models_provider_name_unique";--> statement-breakpoint
ALTER TABLE "models" ADD COLUMN "version" text DEFAULT '' NOT NULL;--> statement-breakpoint
ALTER TABLE "models" ADD COLUMN "input_schema" jsonb;--> statement-breakpoint
ALTER TABLE "models" ADD COLUMN "output_schema" jsonb;--> statement-breakpoint
ALTER TABLE "models" ADD COLUMN "config" jsonb DEFAULT '{}' NOT NULL;--> statement-breakpoint
ALTER TABLE "document_segmentations" ADD CONSTRAINT "document_segmentations_source_document_id_documents_id_fk" FOREIGN KEY ("source_document_id") REFERENCES "public"."documents"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "document_segmentations" ADD CONSTRAINT "document_segmentations_segmented_document_id_documents_id_fk" FOREIGN KEY ("segmented_document_id") REFERENCES "public"."documents"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "document_segmentations" ADD CONSTRAINT "document_segmentations_model_id_models_id_fk" FOREIGN KEY ("model_id") REFERENCES "public"."models"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "document_segmentations_source_document_id_idx" ON "document_segmentations" USING btree ("source_document_id");--> statement-breakpoint
CREATE INDEX "document_segmentations_segmented_document_id_idx" ON "document_segmentations" USING btree ("segmented_document_id");--> statement-breakpoint
CREATE INDEX "document_segmentations_model_id_idx" ON "document_segmentations" USING btree ("model_id");--> statement-breakpoint
CREATE INDEX "document_segmentations_source_document_model_id_idx" ON "document_segmentations" USING btree ("source_document_id","model_id");--> statement-breakpoint
CREATE UNIQUE INDEX "models_provider_name_version_unique" ON "models" USING btree ("provider","name","version");