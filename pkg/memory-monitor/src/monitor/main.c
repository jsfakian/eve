// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

#include <errno.h>
#include <fcntl.h>
#include <getopt.h>
#include <libgen.h>
#include <linux/limits.h>
#include <semaphore.h>
#include <signal.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <syslog.h>
#include <unistd.h>

#include "cgroups.h"
#include "config.h"

#include "monitor.h"

volatile bool syslog_opened = false;
volatile bool semaphores_initialized = false;
resources_to_cleanup_t resources_to_cleanup = {{NULL,0}, {NULL, 0}};
sem_t reload_semaphore;

int handler_log_fd_g = -1;
char binary_location_g[PATH_MAX + 1];

void onreload_cleanup() {
    // Stop the threads
    if (resources_to_cleanup.threads_to_finish.threads != NULL) {
        for (size_t i = 0; i < resources_to_cleanup.threads_to_finish.count; i++) {
            pthread_cancel(resources_to_cleanup.threads_to_finish.threads[i]);
        }
        free(resources_to_cleanup.threads_to_finish.threads);
        resources_to_cleanup.threads_to_finish.threads = NULL;
        resources_to_cleanup.threads_to_finish.count = 0;
    }
    // Close the FDs
    if (resources_to_cleanup.fds_to_close.fds != NULL) {
        for (size_t i = 0; i < resources_to_cleanup.fds_to_close.count; i++) {
            if (resources_to_cleanup.fds_to_close.fds[i] != -1) {
                close(resources_to_cleanup.fds_to_close.fds[i]);
            }
        }
        free(resources_to_cleanup.fds_to_close.fds);
        resources_to_cleanup.fds_to_close.fds = NULL;
        resources_to_cleanup.fds_to_close.count = 0;
    }
}


void onexit_cleanup() {
    // Stop the threads and close the FDs
    onreload_cleanup();
    // Destroy the semaphore
    if (semaphores_initialized) {
        sem_destroy(&reload_semaphore);
    }
    // Close the system log
    if (syslog_opened) {
        syslog(LOG_INFO, "Stopping\n");
        closelog();
    }
}

void term_handler(int signo) {
    exit(signo); // This will cause the onexit_cleanup func to be called, closing the system log and cleaning up the resources
}

void hup_handler(int signo) {
    (void)signo; // Unused
    int backup_errno = errno;
    sem_post(&reload_semaphore);
    // Stop the thread
    onreload_cleanup();
    errno = backup_errno;
}

int main(int argc, char *argv[]) {
    pid_t pid, sid;
    bool daemonize = true;
    int opt;

    while ((opt = getopt(argc, argv, "f")) != -1) {
        switch (opt) {
            case 'f':
                daemonize = false;
                break;
            default:
                exit(EXIT_FAILURE);
        }
    }

    if (daemonize) {
        // Fork off the parent process
        pid = fork();
        if (pid < 0) {
            exit(EXIT_FAILURE);
        }
        // If we got a good PID, then we can exit the parent process
        if (pid > 0) {
            exit(EXIT_SUCCESS);
        }

        // Change the file mode mask
        umask(0);

        // Create a new SID for the child process
        sid = setsid();
        if (sid < 0) {
            exit(EXIT_FAILURE);
        }
    }

    // Move the process to the root cgroup
    cgroup_move_process_to_root_memory(getpid());

    // Save the binary location, as the handler script is in the same directory
    // First, get the full path to the binary
    char binary_full_path[PATH_MAX + 1];
    if (realpath(argv[0], binary_full_path) == NULL) {
        exit(EXIT_FAILURE);
    }
    //Then - copy the directory part into the global variable
    strncpy(binary_location_g, dirname(binary_full_path), sizeof(binary_location_g) - 1);

    // Create a new application directory, if it doesn't exist
    if (access(APP_DIR, F_OK) == -1) {
        if (mkdir(APP_DIR, 0755) == -1) {
            exit(EXIT_FAILURE);
        }
    }

    // Change the current working directory
    if ((chdir(APP_DIR)) < 0) {
        exit(EXIT_FAILURE);
    }

    // Close the standard file descriptors
    close(STDIN_FILENO);
    close(STDOUT_FILENO);
    close(STDERR_FILENO);

    // Create the log directory if it doesn't exist
    if (access(LOG_DIR, F_OK) == -1) {
        if (mkdir(LOG_DIR, 0755) == -1) {
            syslog(LOG_ERR, "Failed to create log directory: %s", strerror(errno));
            exit(EXIT_FAILURE);
        }
    }

    // Redirect the standard file descriptors to a dedicated file
    handler_log_fd_g = open(LOG_DIR "/" HANDLER_LOG_FILE, O_WRONLY | O_CREAT | O_APPEND, S_IRUSR | S_IWUSR | S_IRGRP | S_IROTH);
    if (handler_log_fd_g == -1) {
        exit(EXIT_FAILURE);
    }
    dup2(handler_log_fd_g, STDOUT_FILENO);
    dup2(handler_log_fd_g, STDERR_FILENO);
    if (handler_log_fd_g > STDERR_FILENO) {
        close(handler_log_fd_g);
    }

    // Set the signal handler for signals sent to kill the process
    // We need to call exit() in the handler to close the system log and clean up the resources
    if (signal(SIGTERM, term_handler) == SIG_ERR) {
        exit(EXIT_FAILURE);
    }
    atexit(onexit_cleanup);

    // Initialize the semaphore to 1 to start the monitor immediately
    sem_init(&reload_semaphore, 0, 1);
    // Set the signal handler to reload the config and restart the monitor
    if (signal(SIGHUP, hup_handler) == SIG_ERR) {
        exit(EXIT_FAILURE);
    }

    // Open the system log
    openlog("memory-monitor", LOG_PID | LOG_NDELAY, LOG_DAEMON);
    syslog_opened = true;

    syslog(LOG_INFO, "Starting\n");

    config_t config;

    // Main loop, reload the config and restart the monitor when a signal is received
    while (true) {
        // Sleep until a signal is received
        sem_wait(&reload_semaphore);

        config_read(&config);
        config_validate(&config);

        if (monitor_start(&config, &resources_to_cleanup) != 0) {
            syslog(LOG_ERR, "Failed to run the monitor\n");
            exit(EXIT_FAILURE);
        }
    }

    // We should never reach this point, if we do, something went wrong
    return 1;
}
