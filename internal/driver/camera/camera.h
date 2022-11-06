#include <stdlib.h>
#include "ASICamera2.h"

#ifndef __CAMERA_H
#define __CAMERA_H

// Wraps camera info and return code in a single struct to handle to golang
typedef struct property_wrapper {
	ASI_CAMERA_INFO info;
	int retcode;
} property_wrapper;

// Call 
property_wrapper wrap_ASIGetCameraProperty(int index);

// Helps building a linked list to pass to golang
typedef struct control_list {
	ASI_CONTROL_CAPS info;
	struct control_list *next;
} control_list;

// Wraps linked list and return code in a single struct to handle to golang
typedef struct control_wrapper {
	control_list *alloc;
	int control_num;
	int retcode;
} control_wrapper;

control_wrapper wrap_ASIGetControlCaps(int camera_id);
void free_control_wrapper(control_wrapper w);

// Helps building a linked list to pass to golang
typedef struct ppid_list {
	int info;
	struct ppid_list *next;
} ppid_list;

typedef struct ppid_wrapper {
	ppid_list *alloc;
	int control_num;
	int retcode;
} ppid_wrapper;

ppid_wrapper wrap_ASIGetProductIDs();
void free_ppid_wrapper(ppid_wrapper w);

#endif // __CAMERA_H