#!/bin/bash

# check-coverage.sh
# Validates that test coverage meets the minimum threshold requirement
# Usage: ./scripts/check-coverage.sh [coverage-file] [minimum-threshold]

set -euo pipefail

# Default values
COVERAGE_FILE="${1:-coverage.out}"
MIN_THRESHOLD="${2:-80.0}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to extract coverage percentage from go tool cover output
extract_coverage() {
    local coverage_output="$1"
    echo "$coverage_output" | grep "total:" | awk '{print $3}' | sed 's/%//'
}

# Function to compare coverage percentages (handles floating point)
compare_coverage() {
    local actual="$1"
    local required="$2"
    # Use awk for floating point comparison
    awk -v actual="$actual" -v required="$required" 'BEGIN { exit !(actual >= required) }'
}

# Function to generate detailed coverage report
generate_detailed_report() {
    local coverage_file="$1"
    local coverage_output="$2"
    
    log_info "Generating detailed coverage report..."
    
    echo
    echo "=== COVERAGE BREAKDOWN BY PACKAGE ==="
    echo "$coverage_output" | grep -v "total:" | while read -r line; do
        if [[ -n "$line" ]]; then
            package=$(echo "$line" | awk '{print $1}')
            coverage=$(echo "$line" | awk '{print $3}')
            
            # Color code based on coverage level
            coverage_num="${coverage%\%}"
            if [[ "$coverage_num" == "0.0" ]]; then
                echo -e "${RED}  $package: $coverage${NC}"
            elif awk -v val="$coverage_num" 'BEGIN { exit !(val < 50) }'; then
                echo -e "${RED}  $package: $coverage${NC}"
            elif awk -v val="$coverage_num" 'BEGIN { exit !(val < 80) }'; then
                echo -e "${YELLOW}  $package: $coverage${NC}"
            else
                echo -e "${GREEN}  $package: $coverage${NC}"
            fi
        fi
    done
    echo
}

# Function to generate coverage summary
generate_summary() {
    local actual_coverage="$1"
    local min_threshold="$2"
    local coverage_file="$3"
    
    echo "=== COVERAGE SUMMARY ==="
    echo "Coverage File: $coverage_file"
    echo "Minimum Required: ${min_threshold}%"
    echo "Actual Coverage: ${actual_coverage}%"
    
    if compare_coverage "$actual_coverage" "$min_threshold"; then
        local margin=$(awk -v actual="$actual_coverage" -v required="$min_threshold" 'BEGIN { printf "%.1f", actual - required }')
        echo -e "Status: ${GREEN}PASS${NC} (${margin}% above threshold)"
    else
        local deficit=$(awk -v actual="$actual_coverage" -v required="$min_threshold" 'BEGIN { printf "%.1f", required - actual }')
        echo -e "Status: ${RED}FAIL${NC} (${deficit}% below threshold)"
    fi
    echo
}

# Function to suggest improvements for low coverage packages
suggest_improvements() {
    local coverage_output="$1"
    local low_coverage_packages=()
    
    while read -r line; do
        if [[ -n "$line" && "$line" != *"total:"* ]]; then
            package=$(echo "$line" | awk '{print $1}')
            coverage=$(echo "$line" | awk '{print $3}' | sed 's/%//')
            
            if awk -v val="$coverage" 'BEGIN { exit !(val < 80) }'; then
                low_coverage_packages+=("$package ($coverage%)")
            fi
        fi
    done <<< "$coverage_output"
    
    if [[ ${#low_coverage_packages[@]} -gt 0 ]]; then
        log_warning "Packages with coverage below 80%:"
        for pkg in "${low_coverage_packages[@]}"; do
            echo "  - $pkg"
        done
        echo
        echo "Suggestions for improvement:"
        echo "  1. Add unit tests for uncovered functions"
        echo "  2. Add error scenario testing"
        echo "  3. Add integration tests for complex workflows"
        echo "  4. Review and test edge cases"
        echo
    fi
}

# Main execution
main() {
    log_info "VoidRunner Coverage Threshold Checker"
    log_info "======================================"
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    # Check if coverage file exists
    if [[ ! -f "$COVERAGE_FILE" ]]; then
        log_error "Coverage file '$COVERAGE_FILE' not found!"
        log_info "Please run 'make coverage' or 'go test -coverprofile=$COVERAGE_FILE ./...' first"
        exit 1
    fi
    
    log_info "Checking coverage from: $COVERAGE_FILE"
    log_info "Minimum threshold: ${MIN_THRESHOLD}%"
    echo
    
    # Generate coverage report
    coverage_output=$(go tool cover -func="$COVERAGE_FILE")
    
    if [[ -z "$coverage_output" ]]; then
        log_error "Failed to generate coverage report from $COVERAGE_FILE"
        exit 1
    fi
    
    # Extract total coverage percentage
    actual_coverage=$(extract_coverage "$coverage_output")
    
    if [[ -z "$actual_coverage" ]]; then
        log_error "Could not extract coverage percentage from report"
        exit 1
    fi
    
    # Generate detailed report
    generate_detailed_report "$COVERAGE_FILE" "$coverage_output"
    
    # Generate summary
    generate_summary "$actual_coverage" "$MIN_THRESHOLD" "$COVERAGE_FILE"
    
    # Check if coverage meets threshold
    if compare_coverage "$actual_coverage" "$MIN_THRESHOLD"; then
        log_success "Coverage threshold check PASSED!"
        
        # Show suggestions for packages that could be improved
        suggest_improvements "$coverage_output"
        
        # Generate HTML report if tool is available
        if command -v go >/dev/null 2>&1; then
            html_file="coverage.html"
            go tool cover -html="$COVERAGE_FILE" -o "$html_file" 2>/dev/null
            if [[ -f "$html_file" ]]; then
                log_info "HTML coverage report generated: $html_file"
            fi
        fi
        
        exit 0
    else
        log_error "Coverage threshold check FAILED!"
        suggest_improvements "$coverage_output"
        exit 1
    fi
}

# Handle command line arguments
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    echo "Usage: $0 [coverage-file] [minimum-threshold]"
    echo
    echo "Arguments:"
    echo "  coverage-file      Path to coverage.out file (default: coverage.out)"
    echo "  minimum-threshold  Minimum coverage percentage required (default: 80.0)"
    echo
    echo "Examples:"
    echo "  $0                           # Use defaults"
    echo "  $0 coverage.out 85.0         # Custom threshold"
    echo "  $0 my-coverage.out           # Custom file, default threshold"
    echo
    exit 0
fi

# Run main function
main "$@"