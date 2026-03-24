package interpreters

import "testing"

// SCANNER-4-4: Playwright Test Extraction
func TestPlaywrightInterpreter_Supports(t *testing.T) {
	p := Playwright{}
	for _, f := range []string{"login.spec.ts", "login.spec.js"} {
		if !p.Supports(f) {
			t.Errorf("should support %q", f)
		}
	}
	for _, f := range []string{"foo.test.js", "foo.go", "foo.java"} {
		if p.Supports(f) {
			t.Errorf("should not support %q", f)
		}
	}
}

// SCANNER-4-4: Playwright Test Extraction
func TestPlaywrightInterpreter_TestCall(t *testing.T) {
	src := []byte(`import { test, expect } from '@playwright/test';

test('has title', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Home/);
});

test('has login', async ({ page }) => {
  await page.goto('/login');
});
`)
	funcs, err := Playwright{}.ExtractTestFunctions("login.spec.ts", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 2 {
		t.Fatalf("got %d funcs, want 2", len(funcs))
	}
	if funcs[0].Name != "has title" {
		t.Errorf("funcs[0].Name = %q, want %q", funcs[0].Name, "has title")
	}
	if funcs[0].Line != 3 {
		t.Errorf("funcs[0].Line = %d, want 3", funcs[0].Line)
	}
	if funcs[1].Name != "has login" {
		t.Errorf("funcs[1].Name = %q, want %q", funcs[1].Name, "has login")
	}
}

func TestPlaywrightInterpreter_NoImport(t *testing.T) {
	src := []byte(`test('not playwright', async () => {});
`)
	funcs, err := Playwright{}.ExtractTestFunctions("foo.spec.ts", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funcs) != 0 {
		t.Errorf("got %d funcs, want 0 (no @playwright/test import)", len(funcs))
	}
}
