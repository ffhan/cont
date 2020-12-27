#include <stdio.h>
#include <stdlib.h>

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

void nsexec(void) {
    printf("Hello world from nsenter!\n");
    int pipenum;

    pipenum = initPipe();
    if (pipenum == -1) return;

    printf("executing nsenter...\n");

    fflush(stdout);
}