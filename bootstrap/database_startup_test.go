package bootstrap

import "testing"

func TestAutoMigrateOnStartupDefaultsOff(t *testing.T) {
	t.Setenv("AUTO_MIGRATE_ON_STARTUP", "")
	if autoMigrateOnStartup() {
		t.Fatal("autoMigrateOnStartup() = true by default")
	}
}

func TestAutoMigrateOnStartupAcceptsExplicitOptIn(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", " yes "} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("AUTO_MIGRATE_ON_STARTUP", value)
			if !autoMigrateOnStartup() {
				t.Fatalf("autoMigrateOnStartup() = false for %q", value)
			}
		})
	}
}
