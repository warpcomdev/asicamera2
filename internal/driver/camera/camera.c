#include <stdlib.h>
#include "ASICamera2.h"
#include "camera.h"

property_wrapper wrap_ASIGetCameraProperty(int index) {
	property_wrapper prop;
	prop.retcode = ASIGetCameraProperty(&(prop.info), index);
	return prop;
}

control_wrapper wrap_ASIGetControlCaps(int iCameraID) {
	control_wrapper wrapper;
	wrapper.alloc = NULL;
	wrapper.retcode = ASIGetNumOfControls(iCameraID, &wrapper.control_num);
	if (wrapper.retcode != ASI_SUCCESS) {
		return wrapper;
	}
	wrapper.alloc = (control_list*)malloc(wrapper.control_num * sizeof(control_list));
	if (wrapper.alloc == NULL) {
		wrapper.retcode = ASI_ERROR_BUFFER_TOO_SMALL;
		return wrapper;
	}
	for (int idx = 0; idx < wrapper.control_num; idx++) {
		ASI_CONTROL_CAPS w;
		wrapper.retcode = ASIGetControlCaps(iCameraID, idx, &w);
		if (wrapper.retcode != ASI_SUCCESS) {
			free(wrapper.alloc);
			wrapper.alloc = NULL;
			return wrapper;
		}
		wrapper.alloc[idx].info = w; // full struct copy, there are no pointers in ASI_CONTROL_CAPS
		if (idx < wrapper.control_num - 1) {
			wrapper.alloc[idx].next = &(wrapper.alloc[idx+1]);
		} else {
			wrapper.alloc[idx].next = NULL;
		}
	}
	return wrapper;
}

void free_control_wrapper(control_wrapper w) {
	if (w.alloc != NULL) {
		free(w.alloc);
	}
}

ppid_wrapper wrap_ASIGetProductIDs() {
	ppid_wrapper wrapper;
	wrapper.alloc = NULL;
	wrapper.control_num = ASIGetProductIDs(NULL);
	int *alloc = (int*)malloc(wrapper.control_num*sizeof(int));
	if (alloc == NULL) {
		wrapper.retcode = ASI_ERROR_BUFFER_TOO_SMALL;
		return wrapper;
	}
	wrapper.retcode = ASIGetProductIDs(alloc);
	if (wrapper.retcode != ASI_SUCCESS) {
		free(alloc);
		return wrapper;
	}
	wrapper.alloc = (ppid_list *)malloc(wrapper.control_num*sizeof(ppid_list));
	if (wrapper.alloc == NULL) {
		wrapper.retcode = ASI_ERROR_BUFFER_TOO_SMALL;
		free(alloc);
		return wrapper;
	}
	for (int idx = 0; idx < wrapper.control_num; idx++) {
		wrapper.alloc[idx].info = alloc[idx];
		if (idx < wrapper.control_num - 1) {
			wrapper.alloc[idx].next = &(wrapper.alloc[idx+1]);
		} else {
			wrapper.alloc[idx].next = NULL;
		}

	}
	free(alloc);
	return wrapper;
}

void free_ppid_wrapper(ppid_wrapper w) {
	if (w.alloc != NULL) {
		free(w.alloc);
	}
}
