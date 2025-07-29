package logging

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/voidrunnerhq/voidrunner/internal/database"
)

// PostgreSQLLogStorage implements LogStorage using PostgreSQL
type PostgreSQLLogStorage struct {
	db     *database.Connection
	config *LogConfig
	logger *slog.Logger

	// Batching
	batchChan chan LogEntry
	batchWG   sync.WaitGroup

	// Background goroutines
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	totalInserts    int64
	totalBatches    int64
	lastInsertTime  time.Time
	insertErrors    int64
}

// NewPostgreSQLLogStorage creates a new PostgreSQL-based log storage
func NewPostgreSQLLogStorage(db *database.Connection, config *LogConfig, logger *slog.Logger) (*PostgreSQLLogStorage, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	
	if config == nil {
		config = DefaultLogConfig()
	}
	
	if logger == nil {
		logger = slog.Default()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid log config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	storage := &PostgreSQLLogStorage{
		db:        db,
		config:    config,
		logger:    logger.With("component", "log_storage"),
		batchChan: make(chan LogEntry, config.BufferSize),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start background batch processor
	storage.wg.Add(1)
	go storage.batchProcessor()

	// Start background cleanup processor
	storage.wg.Add(1)
	go storage.cleanupProcessor()

	// Start background partition manager
	storage.wg.Add(1)
	go storage.partitionManager()

	return storage, nil
}

// StoreLogs persists a batch of log entries to the database
func (s *PostgreSQLLogStorage) StoreLogs(ctx context.Context, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Validate all entries
	for i, entry := range entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("invalid log entry at index %d: %w", i, err)
		}
	}

	// If we're using batching, add to batch channel
	if s.config.BatchInsertSize > 1 {
		for _, entry := range entries {
			select {
			case s.batchChan <- entry:
				// Added to batch
			case <-ctx.Done():
				return ctx.Err()
			case <-s.ctx.Done():
				return fmt.Errorf("storage service is shutting down")
			default:
				// Channel is full, fall back to direct insert
				return s.insertLogsBatch(ctx, entries)
			}
		}
		return nil
	}

	// Direct insert for single entries or when batching is disabled
	return s.insertLogsBatch(ctx, entries)
}

// GetLogs retrieves historical logs with filtering and pagination
func (s *PostgreSQLLogStorage) GetLogs(ctx context.Context, filter LogFilter) ([]LogEntry, error) {
	if err := filter.Validate(); err != nil {
		return nil, fmt.Errorf("invalid log filter: %w", err)
	}

	query := `
		SELECT id, task_id, execution_id, content, stream, sequence_number, timestamp, created_at
		FROM task_logs
		WHERE task_id = $1
	`
	args := []interface{}{filter.TaskID}
	argIndex := 2

	// Add optional filters
	if filter.ExecutionID != nil {
		query += fmt.Sprintf(" AND execution_id = $%d", argIndex)
		args = append(args, *filter.ExecutionID)
		argIndex++
	}

	if filter.Stream != "" {
		query += fmt.Sprintf(" AND stream = $%d", argIndex)
		args = append(args, filter.Stream)
		argIndex++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, *filter.EndTime)
		argIndex++
	}

	// Add ordering and pagination
	query += " ORDER BY sequence_number ASC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		err := rows.Scan(
			&log.ID,
			&log.TaskID,
			&log.ExecutionID,
			&log.Content,
			&log.Stream,
			&log.SequenceNumber,
			&log.Timestamp,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating log rows: %w", err)
	}

	return logs, nil
}

// SearchLogs performs full-text search on log content
func (s *PostgreSQLLogStorage) SearchLogs(ctx context.Context, taskID uuid.UUID, query string, limit int) ([]LogEntry, error) {
	if taskID == uuid.Nil {
		return nil, fmt.Errorf("task_id cannot be empty")
	}

	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	sqlQuery := `
		SELECT id, task_id, execution_id, content, stream, sequence_number, timestamp, created_at
		FROM task_logs
		WHERE task_id = $1
		AND to_tsvector('english', content) @@ plainto_tsquery('english', $2)
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := s.db.Query(ctx, sqlQuery, taskID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search logs: %w", err)
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		err := rows.Scan(
			&log.ID,
			&log.TaskID,
			&log.ExecutionID,
			&log.Content,
			&log.Stream,
			&log.SequenceNumber,
			&log.Timestamp,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return logs, nil
}

// GetLogCount returns the total number of log entries for a task/execution
func (s *PostgreSQLLogStorage) GetLogCount(ctx context.Context, taskID uuid.UUID, executionID *uuid.UUID) (int64, error) {
	if taskID == uuid.Nil {
		return 0, fmt.Errorf("task_id cannot be empty")
	}

	query := "SELECT COUNT(*) FROM task_logs WHERE task_id = $1"
	args := []interface{}{taskID}

	if executionID != nil {
		query += " AND execution_id = $2"
		args = append(args, *executionID)
	}

	var count int64
	err := s.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count logs: %w", err)
	}

	return count, nil
}

// CleanupOldLogs removes logs older than the retention period
func (s *PostgreSQLLogStorage) CleanupOldLogs(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("retention_days must be positive")
	}

	result, err := s.db.QueryRow(ctx, "SELECT cleanup_old_task_logs_partitions($1)", retentionDays)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	var resultText string
	if err := result.Scan(&resultText); err != nil {
		return 0, fmt.Errorf("failed to read cleanup result: %w", err)
	}

	s.logger.Info("log cleanup completed", "result", resultText)

	// For simplicity, return 0 for now. In a real implementation,
	// you might want to parse the result text to extract the actual count
	return 0, nil
}

// CreatePartition creates a new daily partition for the specified date
func (s *PostgreSQLLogStorage) CreatePartition(ctx context.Context, date time.Time) error {
	result, err := s.db.QueryRow(ctx, "SELECT create_task_logs_partition($1)", date)
	if err != nil {
		return fmt.Errorf("failed to create partition: %w", err)
	}

	var resultText string
	if err := result.Scan(&resultText); err != nil {
		return fmt.Errorf("failed to read partition creation result: %w", err)
	}

	s.logger.Info("partition creation result", "result", resultText, "date", date.Format("2006-01-02"))
	return nil
}

// GetPartitionStats returns statistics about log partitions
func (s *PostgreSQLLogStorage) GetPartitionStats(ctx context.Context) ([]PartitionStats, error) {
	query := `
		SELECT partition_name, row_count, size_bytes, partition_date
		FROM get_task_logs_partition_stats()
		ORDER BY partition_date DESC
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get partition stats: %w", err)
	}
	defer rows.Close()

	var stats []PartitionStats
	for rows.Next() {
		var stat PartitionStats
		err := rows.Scan(
			&stat.PartitionName,
			&stat.RowCount,
			&stat.SizeBytes,
			&stat.PartitionDate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan partition stats: %w", err)
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating partition stats: %w", err)
	}

	return stats, nil
}

// Close shuts down the storage service and cleans up connections
func (s *PostgreSQLLogStorage) Close() error {
	s.logger.Info("shutting down log storage service")

	// Cancel context to stop background goroutines
	s.cancel()

	// Close batch channel
	close(s.batchChan)

	// Wait for batch processor to finish
	s.batchWG.Wait()

	// Wait for other background goroutines
	s.wg.Wait()

	s.logger.Info("log storage service shutdown complete")
	return nil
}

// batchProcessor processes log entries in batches
func (s *PostgreSQLLogStorage) batchProcessor() {
	defer s.wg.Done()

	batch := make([]LogEntry, 0, s.config.BatchInsertSize)
	ticker := time.NewTicker(s.config.BatchInsertInterval)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-s.batchChan:
			if !ok {
				// Channel closed, process final batch
				if len(batch) > 0 {
					if err := s.insertLogsBatch(s.ctx, batch); err != nil {
						s.logger.Error("failed to insert final batch", "error", err, "size", len(batch))
					}
				}
				return
			}

			batch = append(batch, entry)

			// Insert batch if it's full
			if len(batch) >= s.config.BatchInsertSize {
				if err := s.insertLogsBatch(s.ctx, batch); err != nil {
					s.logger.Error("failed to insert batch", "error", err, "size", len(batch))
					s.insertErrors++
				}
				batch = batch[:0] // Reset batch
			}

		case <-ticker.C:
			// Insert batch on timer if it has entries
			if len(batch) > 0 {
				if err := s.insertLogsBatch(s.ctx, batch); err != nil {
					s.logger.Error("failed to insert timed batch", "error", err, "size", len(batch))
					s.insertErrors++
				}
				batch = batch[:0] // Reset batch
			}

		case <-s.ctx.Done():
			// Process final batch before shutdown
			if len(batch) > 0 {
				if err := s.insertLogsBatch(s.ctx, batch); err != nil {
					s.logger.Error("failed to insert shutdown batch", "error", err, "size", len(batch))
				}
			}
			return
		}
	}
}

// insertLogsBatch inserts a batch of log entries
func (s *PostgreSQLLogStorage) insertLogsBatch(ctx context.Context, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Use COPY for better performance with large batches
	if len(entries) > 10 {
		return s.insertLogsBatchCopy(ctx, entries)
	}

	// Use regular INSERT for small batches
	return s.insertLogsBatchInsert(ctx, entries)
}

// insertLogsBatchInsert uses regular INSERT statements
func (s *PostgreSQLLogStorage) insertLogsBatchInsert(ctx context.Context, entries []LogEntry) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	query := `
		INSERT INTO task_logs (task_id, execution_id, content, stream, sequence_number, timestamp, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	for _, entry := range entries {
		_, err = tx.Exec(ctx, query,
			entry.TaskID,
			entry.ExecutionID,
			entry.Content,
			entry.Stream,
			entry.SequenceNumber,
			entry.Timestamp,
			entry.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert log entry: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.totalInserts += int64(len(entries))
	s.totalBatches++
	s.lastInsertTime = time.Now()

	s.logger.Debug("inserted log batch", "size", len(entries))
	return nil
}

// insertLogsBatchCopy uses COPY for better performance
func (s *PostgreSQLLogStorage) insertLogsBatchCopy(ctx context.Context, entries []LogEntry) error {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// Use COPY for bulk insert
	rows := make([][]interface{}, len(entries))
	for i, entry := range entries {
		rows[i] = []interface{}{
			entry.TaskID,
			entry.ExecutionID,
			entry.Content,
			entry.Stream,
			entry.SequenceNumber,
			entry.Timestamp,
			entry.CreatedAt,
		}
	}

	columns := []string{"task_id", "execution_id", "content", "stream", "sequence_number", "timestamp", "created_at"}
	copyCount, err := conn.CopyFrom(ctx, pgx.Identifier{"task_logs"}, columns, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("failed to copy log entries: %w", err)
	}

	s.totalInserts += copyCount
	s.totalBatches++
	s.lastInsertTime = time.Now()

	s.logger.Debug("copied log batch", "size", copyCount)
	return nil
}

// cleanupProcessor runs periodic cleanup operations
func (s *PostgreSQLLogStorage) cleanupProcessor() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.CleanupOldLogs(s.ctx, s.config.RetentionDays); err != nil {
				s.logger.Error("failed to cleanup old logs", "error", err)
			}
		}
	}
}

// partitionManager creates new partitions as needed
func (s *PostgreSQLLogStorage) partitionManager() {
	defer s.wg.Done()

	ticker := time.NewTicker(24 * time.Hour) // Check daily
	defer ticker.Stop()

	// Create initial partitions
	s.createUpcomingPartitions()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.createUpcomingPartitions()
		}
	}
}

// createUpcomingPartitions creates partitions for upcoming days
func (s *PostgreSQLLogStorage) createUpcomingPartitions() {
	today := time.Now()
	
	for i := 0; i < s.config.PartitionCreationDays; i++ {
		date := today.AddDate(0, 0, i)
		if err := s.CreatePartition(s.ctx, date); err != nil {
			s.logger.Error("failed to create partition", "error", err, "date", date.Format("2006-01-02"))
		}
	}
}