package skills_test

import (
	"testing"

	"abot/pkg/skills"
	"abot/pkg/skills/clawhub"
)

func TestSlugPattern(t *testing.T) {
	valid := []string{"my-skill", "skill", "a-b-c", "Skill123"}
	for _, s := range valid {
		if !clawhub.SlugPattern.MatchString(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	invalid := []string{"", "-bad", "bad-", "has space", "a--b", "a/b"}
	for _, s := range invalid {
		if clawhub.SlugPattern.MatchString(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestDeref(t *testing.T) {
	s := "hello"
	if clawhub.Deref(&s) != "hello" {
		t.Error("Deref pointer failed")
	}
	if clawhub.Deref(nil) != "" {
		t.Error("Deref nil should return empty")
	}
}

func TestRegistry_Name(t *testing.T) {
	r := clawhub.New(skills.ClawHubConfig{})
	if r.Name() != "clawhub" {
		t.Errorf("Name: %q", r.Name())
	}
}
