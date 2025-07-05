# Bash execution environment
FROM alpine:latest

# Alpine images use /bin/sh (ash) by default. If true bash is needed:
# RUN apk add --no-cache bash

# Create a non-root user named "executor" with UID/GID 1000
# -D: Don't assign a password
# -u 1000: Set UID to 1000
RUN adduser -D -u 1000 executor

# Switch to the non-root user
USER executor

# Set the working directory
WORKDIR /tmp/workspace

# Potential future enhancements:
# - Install common shell utilities if needed
# - Set specific environment variables

# Default command can be overridden at runtime by Docker client
# For bash, if installed: CMD ["/bin/bash"]
# For sh (default Alpine shell): CMD ["/bin/sh"]
