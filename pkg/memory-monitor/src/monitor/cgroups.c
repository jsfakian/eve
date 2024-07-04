// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <syslog.h>
#include <unistd.h>

#include "config.h"
#include "event.h"
#include "monitor.h"
#include "util.h"

#include "cgroups.h"

// The longest line in the memory.stat file is the line with a key like "hierarchical_workingset_activate_anon" (37 bytes)
// and the longest value is the dec representation of a 64-bit integer "18446744073709551615" (20 bytes), so the longest
// line is 37 + 1 (for space) + 20 + 1 (for the newline character) = 58 bytes.
// So, 256 is more than a safe value for the line length.
// That's true for Linux kernel 6.1, and will be true for the future versions, as the kernel developers are unlikely to
// change the format of the memory.stat file.
#define MAX_MEMORY_STAT_LINE_LENGTH 256

// The maximum length of the memory.usage_in_bytes / memory.limit_in_bytes file is the length of a 65-bit integer,
// which is 20 bytes, plus the newline character, so 21 bytes is a safe value for the string length.
// To be extra safe, we set the value to 256.
#define MAX_MEMORY_STRING_LENGTH 256

unsigned long cgroup_get_memory_usage(const char *cgroup_name) {
    char path[PATH_MAX + 1];
    int fd;
    char str_usage[MAX_MEMORY_STRING_LENGTH];
    unsigned long usage_bytes;
    int printed;

    printed = snprintf(path, sizeof(path), "%s/%s/memory.usage_in_bytes", CGROUP_PATH_PREFIX, cgroup_name);
    if (printed < 0 || printed >= sizeof(path)) {
        syslog(LOG_ERR, "Path to the usage_in_bytes file is too long\n");
        return 0;
    }
    fd = open(path, O_RDONLY);
    if (fd == -1) {
        syslog(LOG_ERR, "opening usage_in_bytes: %s", strerror(errno));
        return 0;
    }

    ssize_t nread = read(fd, str_usage, sizeof(str_usage) - 1);
    if (nread <= 0) {
        syslog(LOG_ERR, "reading usage_in_bytes: %s", strerror(errno));
        close(fd);
        return 0;
    }

    str_usage[nread] = '\0';
    bool error;
    usage_bytes = strtoudec(str_usage, &error);
    if (error) {
        syslog(LOG_ERR, "strtoul: %s", strerror(errno));
        close(fd);
        return 0;
    }

    close(fd);
    return usage_bytes;
}


unsigned long cgroup_get_total_cache(const char *cgroup_name) {
    char path_str[PATH_MAX + 1];
    FILE *file;
    char line[MAX_MEMORY_STAT_LINE_LENGTH];
    unsigned long total_cache = 0;
    int printed;

    // Construct the path to the memory.stat file
    printed = snprintf(path_str, sizeof(path_str), "%s/%s/memory.stat", CGROUP_PATH_PREFIX, cgroup_name);
    if (printed < 0 || printed >= sizeof(path_str)) {
        syslog(LOG_ERR, "Path to the memory.stat file is too long\n");
        return 0;
    }

    // Open the file
    file = fopen(path_str, "r");
    if (file == NULL) {
        syslog(LOG_ERR, "opening memory.stat: %s", strerror(errno));
        return 0;
    }

    // Read the file line by line
    while (fgets(line, sizeof(line), file) != NULL) {
        // If the line starts with "total_cache", extract the value
        if (strncmp(line, "total_cache", strlen("total_cache")) == 0) {
            // Skip the "total_cache " prefix
            char *startptr = line + strlen("total_cache ");
            bool error;
            total_cache = strtoudec(startptr, &error);
            if (error) {
                syslog(LOG_ERR, "strtoul: %s", strerror(errno));
                total_cache = 0;
            }
            break;
        }
    }

    // Close the file
    fclose(file);

    return total_cache;
}

int cgroup_validate(const char *cgroup_name) {
    char path_str[PATH_MAX + 1];
    int fd;
    int printed;

    // Check if the cgroup exists
    printed = snprintf(path_str, sizeof(path_str), "%s/%s", CGROUP_PATH_PREFIX, cgroup_name);
    if (printed < 0 || printed >= sizeof(path_str)) {
        syslog(LOG_ERR, "Path to the cgroup is too long\n");
        return 1;
    }
    fd = open(path_str, O_RDONLY);
    if (fd == -1) {
        syslog(LOG_ERR, "open cgroup: %s", strerror(errno));
        return 1;
    }
    close(fd);

    // Check if the cgroup.event_control file exists
    printed = snprintf(path_str, sizeof(path_str), "%s/%s/cgroup.event_control", CGROUP_PATH_PREFIX, cgroup_name);
    if (printed < 0 || printed >= sizeof(path_str)) {
        syslog(LOG_ERR, "Path to the cgroup.event_control file is too long\n");
        return 1;
    }
    fd = open(path_str, O_RDONLY);
    if (fd == -1) {
        syslog(LOG_ERR, "open cgroup.event_control: %s", strerror(errno));
        syslog(LOG_WARNING, "Note, that event control file is not available on the CONFIG_PREEMPT_RT enabled system\n");
        return 1;
    }
    close(fd);

    // Check if the memory.pressure_level file exists
    printed = snprintf(path_str, sizeof(path_str), "%s/%s/memory.pressure_level", CGROUP_PATH_PREFIX, cgroup_name);
    if (printed < 0 || printed >= sizeof(path_str)) {
        syslog(LOG_ERR, "Path to the memory.pressure_level file is too long\n");
        return 1;
    }
    fd = open(path_str, O_RDONLY);
    if (fd == -1) {
        syslog(LOG_ERR, "open memory.pressure_level: %s", strerror(errno));
        return 1;
    }
    close(fd);

    return 0;
}

int cgroup_get_memory_limit(const char *cgroup_name, unsigned long *limit) {
    char path_str[PATH_MAX + 1];
    int fd;
    ssize_t nread;
    int printed;

    // Open memory.limit_in_bytes file
    printed = snprintf(path_str, sizeof(path_str), "%s/%s/memory.limit_in_bytes", CGROUP_PATH_PREFIX, cgroup_name);
    if (printed < 0 || printed >= sizeof(path_str)) {
        syslog(LOG_ERR, "Path to the limit_in_bytes file is too long\n");
        return -1;
    }
    fd = open(path_str, O_RDONLY);
    if (fd == -1) {
        syslog(LOG_ERR, "opening limit_in_bytes: %s", strerror(errno));
        return -1;
    }

    char str_limit[MAX_MEMORY_STRING_LENGTH];

    // Read the limit
    nread = read(fd, &str_limit, sizeof(str_limit));
    if (nread == -1) {
        syslog(LOG_ERR, "reading limit_in_bytes: %s", strerror(errno));
        close(fd);
        return -1;
    }
    if (nread == 0) {
        syslog(LOG_INFO, "No data read from limit_in_bytes\n");
        close(fd);
        return -1;
    }

    // Remove the newline character
    str_limit[nread] = '\0';
    for (int i = 0; i < nread; i++) {
        if (str_limit[i] < '0' || str_limit[i] > '9' ) {
            str_limit[i] = '\0';
            break;
        }
    }

    // Convert the string to the unsigned long
    bool error;
    *limit = strtoudec(str_limit, &error);
    if (error) {
        syslog(LOG_ERR, "strtoudec: %s", strerror(errno));
        close(fd);
        return -1;
    }

    close(fd);
    return 0;
}

void cgroup_move_process_to_root_memory(int pid) {
    FILE* procs_file = fopen("/sys/fs/cgroup/memory/cgroup.procs", "w");
    if (procs_file == NULL) {
        syslog(LOG_ERR, "Failed to open cgroup.procs file: %s", strerror(errno));
        return;
    }

    if (fprintf(procs_file, "%d\n", pid) < 0) {
        syslog(LOG_ERR, "Failed to write PID to cgroup.procs file: %s", strerror(errno));
    }

    fclose(procs_file);
}

void* cgroups_events_monitor_thread(void *args) {
    event_desc_t *events = ((monitor_cgroups_events_args_t *) args)->events;
    size_t events_count = ((monitor_cgroups_events_args_t *) args)->events_count;

    // We got all the necessary data from the args, so we can free it
    free(args);

    // Use select() to wait for an event
    fd_set event_fds;
    FD_ZERO(&event_fds);

    // Set the fds to the event_fds and find the maximum fd, as required by select()
    int max_fd = -1;
    for (int i = 0; i < events_count; i++) {
        event_desc_t event = events[i];
        if (event.event_fd == -1)
            continue;
        FD_SET(event.event_fd, &event_fds);
        if (event.event_fd > max_fd) {
            max_fd = event.event_fd;
        }
    }

    // Construct cmd to execute the script
    unsigned long usage, cache;

    while (select(max_fd + 1, &event_fds, NULL, NULL, NULL) > 0) {
        uint64_t counter;
        bool handling_necessary = false;
        char event_msg[MAX_EVENT_MSG_LENGTH];

        for (int i = 0; i < events_count; i++) {
            event_desc_t event = events[i];
            if (FD_ISSET(event.event_fd, &event_fds)) {
                // Read from the event fd to clean the counter
                if (read(event.event_fd, &counter, sizeof(counter)) != sizeof(counter)) {
                    syslog(LOG_ERR, "reading event fd: %s", strerror(errno));
                    break;
                }
                if (event.type == THRESHOLD_EVENT) {
                    // If the threshold event occurred, check the memory usage of the cgroup:
                    // exclude the cache from the usage, as the cache is included in the memory usage, but
                    // it can be easily reclaimed by the system, and we don't want to trigger the handler in this case
                    usage = cgroup_get_memory_usage(event.cgroup_name);
                    cache = cgroup_get_total_cache(event.cgroup_name);
                    if (usage - cache >= event.threshold) {
                        syslog(LOG_INFO, "----- %s threshold is reached -----\n", event.cgroup_name);
                        snprintf(event_msg, sizeof(event_msg), "Threshold is reached for cgroup %s: %lu bytes (threshold: %lu bytes)\n",
                                 event.cgroup_name, usage - cache, event.threshold);
                        handling_necessary = true;
                    }
                } else { // PRESSURE_EVENT
                    syslog(LOG_INFO, "----- %s pressure event -----\n", event.cgroup_name);
                    snprintf(event_msg, sizeof(event_msg), "Pressure event for cgroup %s\n", event.cgroup_name);
                    handling_necessary = true;
                }
            }
        }

        // Run the handler script if the threshold is reached, otherwise continue waiting
        if (handling_necessary) {
            // Print timestamp
            int status = run_handler(HANDLER_SCRIPT, event_msg);
            if (status != 0) {
                syslog(LOG_WARNING, "Failed to run the handler script\n");
            }
        }
        FD_ZERO(&event_fds);
        for (int i = 0; i < events_count; i++) {
            event_desc_t event = events[i];
            if (event.event_fd == -1)
                continue;
            FD_SET(event.event_fd, &event_fds);
        }
    }

    // We should never reach this point
    syslog(LOG_ERR, "Exiting the cgroups events monitor thread\n");
    return NULL;
}

bool cgroup_adjust_memory_limit(const char *cgroup_name, int adjust_by_mb)
{
    int printed;
    // Convert the limit in MB to bytes
    long adjust_by_bytes;
    if (convert_mb_to_bytes_signed(adjust_by_mb, &adjust_by_bytes) != 0) {
        syslog(LOG_WARNING, "Invalid memory limit value: %d MB\n", adjust_by_mb);
        return false;
    }

    // Read the current memory limit
    unsigned long current_limit;
    if (cgroup_get_memory_limit(cgroup_name, &current_limit) != 0) {
        syslog(LOG_WARNING, "Failed to read the current memory limit\n");
        return false;
    }

    // Check that the current limit is fit into the signed long, so we can use it in the arithmetic operations
    if (current_limit > LONG_MAX) {
        syslog(LOG_WARNING, "Invalid memory limit value: %lu\n", current_limit);
        return false;
    }

    // Adjust the memory limit by the value of the adjust_by_mb
    // Check if the new limit is within the range of the unsigned long and not less than 0
    long new_limit;
    if (__builtin_saddl_overflow((long)current_limit, adjust_by_bytes, &new_limit)) {
        syslog(LOG_WARNING, "Invalid memory limit value: %lu\n", current_limit);
        return false;
    }

    // Write the new memory limit
    char limit_file[PATH_MAX + 1];
    int fd;
    ssize_t nwritten;

    // Open memory.limit_in_bytes file
    printed = snprintf(limit_file, sizeof(limit_file), "%s/%s/memory.limit_in_bytes", CGROUP_PATH_PREFIX, cgroup_name);
    if (printed < 0 || printed >= sizeof(limit_file)) {
        syslog(LOG_ERR, "Path to the limit_in_bytes file is too long\n");
        return false;
    }
    fd = open(limit_file, O_WRONLY);
    if (fd == -1) {
        syslog(LOG_ERR, "opening limit_in_bytes: %s", strerror(errno));
        return false;
    }

    char str_limit[MAX_MEMORY_STRING_LENGTH];
    snprintf(str_limit, sizeof(str_limit), "%lu", current_limit + adjust_by_bytes);

    // Write the new limit
    nwritten = write(fd, str_limit, strlen(str_limit));
    if (nwritten == -1) {
        syslog(LOG_ERR, "writing limit_in_bytes: %s", strerror(errno));
        close(fd);
        return false;
    }

    close(fd);
    return true;
}
