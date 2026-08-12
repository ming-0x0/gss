---
name: go_tdd
description: Best practices for Test-Driven Development (TDD), table-driven testing, and mock generation in Go applications.
---

# Go Test-Driven Development (TDD) Skill

This skill outlines standard patterns for writing unit tests and using mocks in Go.

## Table-Driven Test Pattern

```go
func TestFeature(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name       string
        input      InputType
        mockSetup  func(m *mocks.MockPort)
        wantErr    bool
        wantResult ResultType
    }{
        {
            name: "success case",
            input: InputType{...},
            mockSetup: func(m *mocks.MockPort) {
                m.EXPECT().Method(gomock.Any(), ...).Return(...)
            },
            wantErr: false,
            wantResult: ...,
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            mockPort := mocks.NewMockPort(ctrl)
            tt.mockSetup(mockPort)

            // Execute service method and assert
        })
    }
}
```

## Rules

1. Always use `t.Parallel()` for fast parallel test execution.
2. Use `gomock` for mocking interfaces in `internal/port/in/mocks` and `internal/port/out/mocks`.
3. Use `httptest.NewRecorder()` and `httptest.NewRequest()` for testing Gin HTTP Handlers.
