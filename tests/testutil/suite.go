package testutil

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
	"github.com/voidrunnerhq/voidrunner/internal/api/routes"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
	"github.com/voidrunnerhq/voidrunner/internal/services"
	"github.com/voidrunnerhq/voidrunner/internal/worker"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

// IntegrationTestSuite provides a comprehensive test suite for integration testing
type IntegrationTestSuite struct {
	suite.Suite
	DB       *DatabaseHelper
	HTTP     *HTTPHelper
	Auth     *AuthHelper
	Factory  *RequestFactory
	Fixtures *AllFixtures
}

// SetupSuite initializes the test suite
func (s *IntegrationTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Initialize database helper
	s.DB = NewDatabaseHelper(s.T())

	// Initialize logger
	log := logger.New("info", "json")

	// Initialize JWT service and auth service
	jwtService := auth.NewJWTService(&s.DB.Config.JWT)
	authService := auth.NewService(
		s.DB.Repositories.Users,
		jwtService,
		log.Logger,
		s.DB.Config,
	)

	// Setup router with full routes
	router := gin.New()

	// For integration tests, we don't need actual queue functionality
	// Use mock queue manager for testing
	mockQueueManager := &MockQueueManager{}
	taskExecutionService := services.NewTaskExecutionService(s.DB.DB, mockQueueManager, log.Logger)

	// Create mock executor for integration tests
	executorConfig := executor.NewDefaultConfig()
	mockExecutor := executor.NewMockExecutor(executorConfig, log.Logger)
	taskExecutorService := services.NewTaskExecutorService(
		taskExecutionService,
		s.DB.Repositories.Tasks,
		mockExecutor,
		nil, // cleanup manager not needed for mock executor
		log.Logger,
	)

	// Create mock worker manager for integration tests (nil since embedded workers disabled in tests)
	var mockWorkerManager worker.WorkerManager = nil

	routes.Setup(router, s.DB.Config, log, s.DB.DB, s.DB.Repositories, authService, taskExecutionService, taskExecutorService, mockWorkerManager)

	// Initialize HTTP helper
	s.HTTP = NewHTTPHelper(router, authService)

	// Initialize auth helper
	s.Auth = NewAuthHelper(authService)

	// Initialize factory
	s.Factory = NewRequestFactory()

	// Initialize fixtures
	s.Fixtures = NewAllFixtures()
}

// TearDownSuite cleans up the test suite
func (s *IntegrationTestSuite) TearDownSuite() {
	if s.DB != nil {
		s.DB.CleanupDatabase(s.T())
		s.DB.Close()
	}
}

// SetupTest runs before each test
func (s *IntegrationTestSuite) SetupTest() {
	// Clean database before each test
	s.DB.CleanupDatabase(s.T())
}

// TearDownTest runs after each test
func (s *IntegrationTestSuite) TearDownTest() {
	// Clean database after each test
	s.DB.CleanupDatabase(s.T())
}

// WithSeededData runs a test function with seeded test data
func (s *IntegrationTestSuite) WithSeededData(testFn func()) {
	s.DB.WithSeededDatabase(s.T(), s.Fixtures, testFn)
}

// CreateAuthenticatedUser creates a user and returns auth context
func (s *IntegrationTestSuite) CreateAuthenticatedUser(email, name string) *AuthContext {
	user := NewUserFactory().
		WithEmail(email).
		WithName(name).
		Build()

	// Create user in database
	err := s.DB.Repositories.Users.Create(context.Background(), user)
	s.Require().NoError(err)

	// Create auth context
	return s.Auth.CreateAuthContext(s.T(), user)
}

// APITestSuite provides a lightweight suite for API-only testing
type APITestSuite struct {
	suite.Suite
	HTTP    *HTTPHelper
	Auth    *AuthHelper
	Factory *RequestFactory
	Config  *config.Config
}

// SetupSuite initializes the API test suite
func (s *APITestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Get test config
	s.Config = GetTestConfig()

	// Create mock repositories if needed for pure API testing
	// For now, skip database setup for API-only tests

	s.Factory = NewRequestFactory()
}

// UnitTestHelper provides utilities for unit testing individual components
type UnitTestHelper struct {
	Factory  *RequestFactory
	Fixtures *AllFixtures
}

// NewUnitTestHelper creates a new unit test helper
func NewUnitTestHelper() *UnitTestHelper {
	return &UnitTestHelper{
		Factory:  NewRequestFactory(),
		Fixtures: NewAllFixtures(),
	}
}

// MockQueueManager provides a mock implementation of queue.QueueManager for testing
type MockQueueManager struct{}

func (m *MockQueueManager) TaskQueue() queue.TaskQueue             { return nil }
func (m *MockQueueManager) RetryQueue() queue.RetryQueue           { return nil }
func (m *MockQueueManager) DeadLetterQueue() queue.DeadLetterQueue { return nil }
func (m *MockQueueManager) Start(ctx context.Context) error        { return nil }
func (m *MockQueueManager) Stop(ctx context.Context) error         { return nil }
func (m *MockQueueManager) IsHealthy(ctx context.Context) error    { return nil }
func (m *MockQueueManager) GetStats(ctx context.Context) (*queue.QueueManagerStats, error) {
	return &queue.QueueManagerStats{}, nil
}
func (m *MockQueueManager) StartRetryProcessor(ctx context.Context) error { return nil }
func (m *MockQueueManager) StopRetryProcessor() error                     { return nil }

// RunIntegrationTests runs integration tests with proper setup
func RunIntegrationTests(t *testing.T, suiteFn func(*IntegrationTestSuite)) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Check if test database is available
	if !isDatabaseAvailable(GetTestConfig()) {
		t.Skip("Test database not available")
	}

	// Create and run suite
	testSuite := &IntegrationTestSuite{}
	suite.Run(t, testSuite)
}

// RunAPITests runs API tests with proper setup
func RunAPITests(t *testing.T, suiteFn func(*APITestSuite)) {
	testSuite := &APITestSuite{}
	suite.Run(t, testSuite)
}

// WithTestDatabase runs a function with a test database
func WithTestDatabase(t *testing.T, testFn func(*DatabaseHelper)) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	db := NewDatabaseHelper(t)
	defer db.Close()

	db.WithCleanDatabase(t, func() {
		testFn(db)
	})
}

// WithSeededTestDatabase runs a function with a seeded test database
func WithSeededTestDatabase(t *testing.T, fixtures *AllFixtures, testFn func(*DatabaseHelper)) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	db := NewDatabaseHelper(t)
	defer db.Close()

	db.WithSeededDatabase(t, fixtures, func() {
		testFn(db)
	})
}

// RunDatabaseTests runs database tests with proper setup
func RunDatabaseTests(t *testing.T, testFn func(*DatabaseHelper)) {
	WithTestDatabase(t, testFn)
}

// BenchmarkHelper provides utilities for benchmark testing
type BenchmarkHelper struct {
	DB      *DatabaseHelper
	Factory *RequestFactory
}

// NewBenchmarkHelper creates a new benchmark helper
func NewBenchmarkHelper(b *testing.B) *BenchmarkHelper {
	// Convert testing.B to testing.T for helper functions
	t := &testing.T{}

	return &BenchmarkHelper{
		DB:      NewDatabaseHelper(t),
		Factory: NewRequestFactory(),
	}
}

// Close closes the benchmark helper
func (h *BenchmarkHelper) Close() {
	if h.DB != nil {
		h.DB.Close()
	}
}

// RunBenchmark runs a benchmark with proper setup
func RunBenchmark(b *testing.B, benchFn func(*BenchmarkHelper)) {
	helper := NewBenchmarkHelper(b)
	defer helper.Close()

	b.ResetTimer()
	benchFn(helper)
}
