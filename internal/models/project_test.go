package models

import "testing"

func TestProjectBeforeCreateAssignsGUID(t *testing.T) {
	project := &Project{}

	if err := project.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if project.GUID == "" {
		t.Fatal("expected BeforeCreate to assign a GUID")
	}
}

func TestProjectBeforeCreatePreservesGUID(t *testing.T) {
	project := &Project{GUID: "existing-guid"}

	if err := project.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if project.GUID != "existing-guid" {
		t.Fatalf("GUID = %q, want existing-guid", project.GUID)
	}
}
