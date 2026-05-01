package security

import "testing"

func TestClassifyCommandBlocksL4(t *testing.T) {
	decision := ClassifyCommand("rm -rf /")
	if decision.Allowed || decision.Level != "L4" {
		t.Fatalf("expected L4 block, got %+v", decision)
	}
}

func TestClassifyCommandRequiresConfirmForRecursiveDelete(t *testing.T) {
	decision := ClassifyCommand(`Remove-Item -Recurse -Force C:\catpaw`)
	if decision.Allowed || decision.Level != "L3" {
		t.Fatalf("expected L3 confirmation, got %+v", decision)
	}
}

func TestClassifyCommandDetectsPowerShellEncodedCommand(t *testing.T) {
	decision := ClassifyCommand("powershell -EncodedCommand cgBtACAALQByAGYAIAAvAA==")
	if decision.Allowed || decision.Level != "L4" {
		t.Fatalf("expected encoded rm -rf / to be L4 block, got %+v", decision)
	}
}

func TestClassifyCommandDetectsCaretObfuscation(t *testing.T) {
	decision := ClassifyCommand(`r^m -r^f /tmp/aiw-danger-sandbox/test`)
	if decision.Allowed || decision.Level != "L4" {
		t.Fatalf("expected obfuscated root-style recursive delete to be blocked, got %+v", decision)
	}
}

func TestValidateRemoteHostWhitelist(t *testing.T) {
	if !ValidateRemoteHost("192.168.1.7").Allowed {
		t.Fatal("expected Windows probe test host to be allowed")
	}
	if ValidateRemoteHost("8.8.8.8").Allowed {
		t.Fatal("expected non-whitelisted host to be rejected")
	}
}

func TestValidatePlatformURLRejectsMetadata(t *testing.T) {
	decision := ValidatePlatformURL("http://169.254.169.254/latest/meta-data")
	if decision.Allowed {
		t.Fatalf("expected metadata URL to be rejected: %+v", decision)
	}
}
