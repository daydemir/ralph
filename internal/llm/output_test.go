package llm

import (
	"strings"
	"testing"
)

func TestSignalDetection(t *testing.T) {
	testCases := []struct {
		name           string
		eventType      string     // "assistant" or "result"
		content        string
		wantSignal     SignalType // empty string for plan_complete, SignalBailout for bailout, others for failure
		isPlanComplete bool       // true if wantSignal is plan_complete
		isBailout      bool       // true if wantSignal is bailout
		wantDetail     string
	}{
		// Text events (assistant type)
		{
			name:           "PLAN_COMPLETE in text",
			eventType:      "assistant",
			content:        "Done! ###PLAN_COMPLETE###",
			isPlanComplete: true,
		},
		{
			name:       "BAILOUT in text",
			eventType:  "assistant",
			content:    "###BAILOUT:context_preservation###",
			wantSignal: SignalBailout,
			isBailout:  true,
			wantDetail: "context_preservation",
		},
		{
			name:       "PLAN_FAILED in text",
			eventType:  "assistant",
			content:    "###PLAN_FAILED:test_infrastructure###",
			wantSignal: SignalPlanFailed,
			wantDetail: "test_infrastructure",
		},
		{
			name:       "TASK_FAILED in text",
			eventType:  "assistant",
			content:    "###TASK_FAILED:build_ios###",
			wantSignal: SignalTaskFailed,
			wantDetail: "build_ios",
		},
		{
			name:       "BLOCKED in text",
			eventType:  "assistant",
			content:    "###BLOCKED:missing_credentials###",
			wantSignal: SignalBlocked,
			wantDetail: "missing_credentials",
		},
		{
			name:       "BUILD_FAILED in text",
			eventType:  "assistant",
			content:    "###BUILD_FAILED:ios###",
			wantSignal: SignalBuildFailed,
			wantDetail: "ios",
		},
		{
			name:       "TEST_FAILED in text",
			eventType:  "assistant",
			content:    "###TEST_FAILED:ios:3###",
			wantSignal: SignalTestFailed,
			wantDetail: "ios:3",
		},
		// Result events - CRITICAL (these were broken before the fix)
		{
			name:       "PLAN_FAILED in result",
			eventType:  "result",
			content:    "Tests failed ###PLAN_FAILED:test_infrastructure### Summary...",
			wantSignal: SignalPlanFailed,
			wantDetail: "test_infrastructure",
		},
		{
			name:       "BAILOUT in result",
			eventType:  "result",
			content:    "###BAILOUT:context_preservation### preserving work",
			wantSignal: SignalBailout,
			isBailout:  true,
			wantDetail: "context_preservation",
		},
		{
			name:       "TEST_FAILED in result",
			eventType:  "result",
			content:    "###TEST_FAILED:ios:3###",
			wantSignal: SignalTestFailed,
			wantDetail: "ios:3",
		},
		{
			name:       "TASK_FAILED in result",
			eventType:  "result",
			content:    "Build failed ###TASK_FAILED:compile_error### see log",
			wantSignal: SignalTaskFailed,
			wantDetail: "compile_error",
		},
		{
			name:       "BLOCKED in result",
			eventType:  "result",
			content:    "###BLOCKED:api_down###",
			wantSignal: SignalBlocked,
			wantDetail: "api_down",
		},
		{
			name:       "BUILD_FAILED in result",
			eventType:  "result",
			content:    "###BUILD_FAILED:backend###",
			wantSignal: SignalBuildFailed,
			wantDetail: "backend",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock stream JSON
			var jsonLine string
			if tc.eventType == "assistant" {
				jsonLine = `{"type":"assistant","message":{"content":[{"type":"text","text":"` + tc.content + `"}]}}`
			} else {
				jsonLine = `{"type":"result","result":"` + tc.content + `"}`
			}

			handler := NewConsoleHandler()
			reader := strings.NewReader(jsonLine + "\n")

			ParseStream(reader, handler, nil)

			// Check signal was detected
			if tc.isPlanComplete {
				if !handler.IsPlanComplete() {
					t.Errorf("Expected plan_complete signal to be detected")
				}
			} else if tc.isBailout {
				if !handler.IsBailout() {
					t.Errorf("Expected bailout signal to be detected")
				}
				if handler.GetBailout() == nil {
					t.Fatalf("GetBailout() returned nil")
				}
				if handler.GetBailout().Detail != tc.wantDetail {
					t.Errorf("Expected bailout detail %q, got %q", tc.wantDetail, handler.GetBailout().Detail)
				}
			} else {
				if !handler.HasFailed() {
					t.Errorf("Expected failure signal %q to be detected", tc.wantSignal)
				}
				fail := handler.GetFailure()
				if fail == nil {
					t.Fatalf("GetFailure() returned nil")
				}
				if fail.Type != tc.wantSignal {
					t.Errorf("Expected signal type %q, got %q", tc.wantSignal, fail.Type)
				}
				if fail.Detail != tc.wantDetail {
					t.Errorf("Expected signal detail %q, got %q", tc.wantDetail, fail.Detail)
				}
			}
		})
	}
}

func TestNoSignalDetected(t *testing.T) {
	// Test that normal text without signals doesn't trigger anything
	jsonLine := `{"type":"assistant","message":{"content":[{"type":"text","text":"Just some normal output without any signals"}]}}`

	handler := NewConsoleHandler()
	reader := strings.NewReader(jsonLine + "\n")

	ParseStream(reader, handler, nil)

	if handler.HasFailed() {
		t.Errorf("Expected no failure signal, got %v", handler.GetFailure())
	}
	if handler.IsBailout() {
		t.Errorf("Expected no bailout signal, got %v", handler.GetBailout())
	}
	if handler.IsPlanComplete() {
		t.Error("Expected no plan_complete signal")
	}
}

func TestOnTerminateCallback(t *testing.T) {
	// Test that onTerminate is called when a failure signal is detected
	called := false
	onTerminate := func() {
		called = true
	}

	jsonLine := `{"type":"result","result":"###PLAN_FAILED:test###"}`

	handler := NewConsoleHandler()
	reader := strings.NewReader(jsonLine + "\n")

	ParseStream(reader, handler, onTerminate)

	if !called {
		t.Error("Expected onTerminate to be called when failure signal detected")
	}
	if !handler.HasFailed() {
		t.Error("Expected failure signal to be recorded")
	}
}
