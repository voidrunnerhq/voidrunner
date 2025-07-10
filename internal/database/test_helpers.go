package database

// Helper functions for pointer types in tests

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
