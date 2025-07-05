# Python execution environment
FROM python:3.11-alpine

# Create a non-root user named "executor" with UID/GID 1000
# -D: Don't assign a password
# -u 1000: Set UID to 1000
RUN adduser -D -u 1000 executor

# Switch to the non-root user
USER executor

# Set the working directory
WORKDIR /tmp/workspace

# Potential future enhancements:
# - Install common Python packages if needed by user code by default
# - Set environment variables specific to Python execution
# - Add healthcheck if this container were to run as a long-lived service (not the case here)

# Default command can be overridden at runtime by Docker client
# CMD ["python3"]
