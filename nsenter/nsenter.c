#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <sched.h>
#include <errno.h>
#include <string.h>
#include <sys/capability.h>
#include <sys/prctl.h>
#include <unistd.h>

static int initPipe(void) {
    int pipenum;
    char *initPipe, *endptr;

    initPipe = getenv("_LIBCONTAINER_INITPIPE");
    if (initPipe == NULL || *initPipe == '\0')
        return -1;

    pipenum = strtol(initPipe, &endptr, 10);
    if (*endptr != '\0') {
        printf("cannot convert string to pipenum\n");
    }
    return pipenum;
}

static void getSharedNSes(int *start, int *end) {
    char *value, *original;
    int result;

    value = getenv("_NS_START");
    if (value == NULL || *value == '\0') {
        printf("no _NS_START\n");
        *start = -1;
        return;
    }
    result = strtol(value, &original, 10);
    if (*original != '\0') {
        *start = -1;
        return;
    }
    *start = result;
    value = getenv("_NS_END");
    if (value == NULL || *value == '\0') {
        printf("no _NS_END\n");
        *end = -1;
        return;
    }
    result = strtol(value, &original, 10);
    if (*original != '\0') {
        *end = -1;
        return;
    }
    *end = result;
}

static int joinNamespaces(int startNSFD, int endNSFD) {
    for(int fd = startNSFD; fd < endNSFD; fd++) {
        if (setns(fd, 0) == -1) {
            printf("cannot setns %d: %s\n", fd, strerror(errno));
            exit(1);
        }
        printf("setns fd %d call success\n", fd);
    }
}

void nsexec(void) {
    cap_t caps;

    printf("Hello world from nsenter (EUID: %d)!\n", geteuid());

    caps = cap_get_proc();
    if (caps == -1) {
        printf("cannot get caps: %s\n", strerror(errno));
    } else {
        printf("caps: %#0X\n", caps);
    }

    if (prctl(PR_SET_DUMPABLE, 1, 0, 0, 0) == -1) {
        printf("cannot set dumpable\n");
        exit(1);
    }

    int pipenum;
    int startNS, endNS;

    pipenum = initPipe();
    if (pipenum == -1) return;

    printf("executing nsenter...\n");

    getSharedNSes(&startNS, &endNS);

    if (startNS != -1 && endNS != -1) {
        printf("sharing NSes from fd %d to %d\n", startNS, endNS);
    }

    joinNamespaces(startNS, endNS);

    printf("done with nsenter!\n");
    fflush(stdout);
}