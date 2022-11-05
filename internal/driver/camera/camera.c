#include <stdlib.h>
#include "ASICamera2.h"
#include "camera.h"

property_wrapper wrap_ASIGetCameraProperty(int index) {
	property_wrapper w;
	w.retcode = ASIGetCameraProperty(&(w.info), index);
	return w;
}

control_wrapper wrap_ASIGetControlCaps(int camera_id) {
	control_wrapper control;
	int retcode = ASIGetNumOfControls(camera_id, &control.control_num);
	if (retcode != ASI_SUCCESS) {
		control.retcode = retcode;
		return control;
	}
	ASI_CONTROL_CAPS w;
	control_list* alloc = (control_list*)malloc(control.control_num * sizeof(control_list));
	if (alloc == NULL) {
		control.retcode = ASI_ERROR_BUFFER_TOO_SMALL;
		return control;
	}
	for (int idx = 0; idx < control.control_num; idx++) {
		retcode = ASIGetControlCaps(camera_id, idx, &w);
		if (retcode != ASI_SUCCESS) {
			control.retcode = retcode;
			free(alloc);
			return control;
		}
		alloc[idx].info = w; // full struct copy, there are no pointers in ASI_CONTROL_CAPS
		if (idx < control.control_num - 1) {
			alloc[idx].next = &alloc[idx+1];
		} else {
			alloc[idx].next = NULL;
		}
	}
	control.retcode = ASI_SUCCESS;
	return control;
}

void free_control_wrapper(control_wrapper w) {
	free(w.alloc);
}
