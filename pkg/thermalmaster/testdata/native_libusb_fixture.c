#include <libusb.h>

#include <errno.h>
#include <fcntl.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

static unsigned char fixture_context_storage;
static unsigned char fixture_device_storage[2];
static unsigned char fixture_handle_storage[2];
static int fixture_system_fd = -1;
static pthread_mutex_t fixture_mutex = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t fixture_changed = PTHREAD_COND_INITIALIZER;
static struct libusb_transfer *fixture_pending;
static int fixture_blocked;
static int fixture_cancelled;

static libusb_device *fixture_device_list[] = {
    (libusb_device *)&fixture_device_storage[0],
    (libusb_device *)&fixture_device_storage[1],
    NULL,
};

static struct libusb_endpoint_descriptor fixture_stream_endpoint = {
    .bLength = LIBUSB_DT_ENDPOINT_SIZE,
    .bDescriptorType = LIBUSB_DT_ENDPOINT,
    .bEndpointAddress = LIBUSB_ENDPOINT_IN | 1,
    .bmAttributes = LIBUSB_TRANSFER_TYPE_BULK,
    .wMaxPacketSize = 512,
};

static struct libusb_interface_descriptor fixture_control_alternates[] = {
    {
        .bLength = LIBUSB_DT_INTERFACE_SIZE,
        .bDescriptorType = LIBUSB_DT_INTERFACE,
        .bInterfaceNumber = 0,
        .bAlternateSetting = 0,
    },
    {
        .bLength = LIBUSB_DT_INTERFACE_SIZE,
        .bDescriptorType = LIBUSB_DT_INTERFACE,
        .bInterfaceNumber = 0,
        .bAlternateSetting = 1,
    },
};

static struct libusb_interface_descriptor fixture_stream_alternates[] = {
    {
        .bLength = LIBUSB_DT_INTERFACE_SIZE,
        .bDescriptorType = LIBUSB_DT_INTERFACE,
        .bInterfaceNumber = 1,
        .bAlternateSetting = 0,
    },
    {
        .bLength = LIBUSB_DT_INTERFACE_SIZE,
        .bDescriptorType = LIBUSB_DT_INTERFACE,
        .bInterfaceNumber = 1,
        .bAlternateSetting = 1,
        .bNumEndpoints = 1,
        .endpoint = &fixture_stream_endpoint,
    },
};

static struct libusb_interface fixture_interfaces[] = {
    {.altsetting = fixture_control_alternates, .num_altsetting = 1},
    {.altsetting = fixture_stream_alternates, .num_altsetting = 2},
};

static struct libusb_config_descriptor fixture_config = {
    .bLength = LIBUSB_DT_CONFIG_SIZE,
    .bDescriptorType = LIBUSB_DT_CONFIG,
    .bNumInterfaces = 2,
    .bConfigurationValue = 1,
    .interface = fixture_interfaces,
};

static int fixture_enabled(const char *name) {
	const char *value = getenv(name);
	return value != NULL && strcmp(value, "1") == 0;
}

static void fixture_log(const char *event) {
	const char *log_path = getenv("THERMALMASTER_NATIVE_FIXTURE_LOG");
	if (log_path == NULL || log_path[0] == '\0') {
		return;
	}
	FILE *log_file = fopen(log_path, "ab");
	if (log_file == NULL) {
		return;
	}
	(void)fputs(event, log_file);
	(void)fputc('\n', log_file);
	(void)fclose(log_file);
}

static int fixture_failure(const char *event) {
	const char *failures = getenv("THERMALMASTER_NATIVE_FIXTURE_FAILURE");
	if (failures == NULL) {
		return LIBUSB_SUCCESS;
	}
	const char *match = strstr(failures, event);
	if (match == NULL || (match != failures && match[-1] != ' ') ||
	    (match[strlen(event)] != '\0' && match[strlen(event)] != ' ')) {
		return LIBUSB_SUCCESS;
	}
	return strcmp(event, "control-alt") == 0 ? LIBUSB_ERROR_PIPE : LIBUSB_ERROR_BUSY;
}

int LIBUSB_CALL libusb_init(libusb_context **ctx) {
	fixture_log("context-init-discovery");
	fixture_system_fd = -1;
	if (fixture_failure("init") != 0) {
		return LIBUSB_ERROR_BUSY;
	}
	*ctx = (libusb_context *)&fixture_context_storage;
	return LIBUSB_SUCCESS;
}

int LIBUSB_CALL libusb_init_context(libusb_context **ctx,
                                     const struct libusb_init_option options[],
                                     int num_options) {
	if (num_options != 1 || options[0].option != LIBUSB_OPTION_NO_DEVICE_DISCOVERY) {
		fixture_log("incorrect-init-options");
		return LIBUSB_ERROR_INVALID_PARAM;
	}
	fixture_log("context-init-no-discovery");
	fixture_system_fd = -1;
	if (fixture_failure("init") != 0) {
		return LIBUSB_ERROR_BUSY;
	}
	*ctx = (libusb_context *)&fixture_context_storage;
	return LIBUSB_SUCCESS;
}

void LIBUSB_CALL libusb_exit(libusb_context *ctx) {
	(void)ctx;
	if (fixture_system_fd >= 0 && fcntl(fixture_system_fd, F_GETFD) != -1) {
		fixture_log("file-open-at-context-exit");
	}
	fixture_log("context-exit");
}

ssize_t LIBUSB_CALL libusb_get_device_list(libusb_context *ctx, libusb_device ***list) {
	(void)ctx;
	fixture_log("device-list");
	*list = fixture_device_list;
	return fixture_enabled("THERMALMASTER_NATIVE_FIXTURE_MULTIPLE") ? 2 : 1;
}

void LIBUSB_CALL libusb_free_device_list(libusb_device **list, int unref_devices) {
	(void)list;
	(void)unref_devices;
	fixture_log("device-list-free");
}

void LIBUSB_CALL libusb_unref_device(libusb_device *dev) {
	(void)dev;
	fixture_log("device-unref");
}

int LIBUSB_CALL libusb_get_device_descriptor(libusb_device *dev,
                                             struct libusb_device_descriptor *desc) {
	(void)dev;
	fixture_log("descriptor");
	if (fixture_failure("descriptor") != 0) {
		return LIBUSB_ERROR_BUSY;
	}
	memset(desc, 0, sizeof(*desc));
	desc->bLength = LIBUSB_DT_DEVICE_SIZE;
	desc->bDescriptorType = LIBUSB_DT_DEVICE;
	desc->bcdUSB = 0x0200;
	desc->idVendor = 0x3474;
	desc->idProduct = 0x45a2;
	desc->iSerialNumber = fixture_enabled("THERMALMASTER_NATIVE_FIXTURE_NO_SERIAL") ? 0 : 1;
	desc->bNumConfigurations = 1;
	return LIBUSB_SUCCESS;
}

int LIBUSB_CALL libusb_get_port_numbers(libusb_device *dev, uint8_t *ports, int length) {
	(void)dev;
	fixture_log("port-numbers");
	if (fixture_failure("port-numbers") != 0) {
		return LIBUSB_ERROR_BUSY;
	}
	if (length < 1) {
		return LIBUSB_ERROR_OVERFLOW;
	}
	ports[0] = 1;
	return 1;
}

int LIBUSB_CALL libusb_get_device_speed(libusb_device *dev) {
	(void)dev;
	return LIBUSB_SPEED_HIGH;
}

uint8_t LIBUSB_CALL libusb_get_bus_number(libusb_device *dev) {
	(void)dev;
	return 7;
}

uint8_t LIBUSB_CALL libusb_get_device_address(libusb_device *dev) {
	return dev == (libusb_device *)&fixture_device_storage[1] ? 10 : 9;
}

int LIBUSB_CALL libusb_open(libusb_device *dev, libusb_device_handle **dev_handle) {
	int second = dev == (libusb_device *)&fixture_device_storage[1];
	if (!second && fixture_enabled("THERMALMASTER_NATIVE_FIXTURE_OPEN_FIRST_FAILS")) {
		fixture_log("handle-open-access-denied");
		return LIBUSB_ERROR_ACCESS;
	}
	fixture_log("handle-open");
	*dev_handle = (libusb_device_handle *)&fixture_handle_storage[second];
	return LIBUSB_SUCCESS;
}

int LIBUSB_CALL libusb_wrap_sys_device(libusb_context *ctx, intptr_t sys_dev,
                                       libusb_device_handle **dev_handle) {
	(void)ctx;
	fixture_log("handle-wrap");
	if (fixture_failure("wrap") != 0) {
		return LIBUSB_ERROR_BUSY;
	}
	if (fcntl((int)sys_dev, F_GETFD) == -1) {
		return LIBUSB_ERROR_INVALID_PARAM;
	}
	fixture_system_fd = (int)sys_dev;
	*dev_handle = (libusb_device_handle *)&fixture_handle_storage[0];
	return LIBUSB_SUCCESS;
}

void LIBUSB_CALL libusb_close(libusb_device_handle *dev_handle) {
	(void)dev_handle;
	if (fixture_system_fd >= 0 && fcntl(fixture_system_fd, F_GETFD) != -1) {
		fixture_log("file-open-at-handle-close");
	}
	fixture_log("handle-close");
}

libusb_device *LIBUSB_CALL libusb_get_device(libusb_device_handle *dev_handle) {
	int second = dev_handle == (libusb_device_handle *)&fixture_handle_storage[1];
	return (libusb_device *)&fixture_device_storage[second];
}

int LIBUSB_CALL libusb_get_config_descriptor(libusb_device *dev, uint8_t index,
                                             struct libusb_config_descriptor **config) {
	(void)dev;
	fixture_log("config-descriptor");
	if (fixture_failure("config-descriptor") != 0) {
		return LIBUSB_ERROR_BUSY;
	}
	if (index != 0) {
		return LIBUSB_ERROR_NOT_FOUND;
	}
	fixture_interfaces[0].num_altsetting = fixture_enabled("THERMALMASTER_NATIVE_FIXTURE_CONTROL_ALTERNATES") ? 2 : 1;
	*config = &fixture_config;
	return LIBUSB_SUCCESS;
}

void LIBUSB_CALL libusb_free_config_descriptor(struct libusb_config_descriptor *config) {
	(void)config;
	fixture_log("config-descriptor-free");
}

int LIBUSB_CALL libusb_get_configuration(libusb_device_handle *dev_handle, int *config) {
	(void)dev_handle;
	fixture_log("get-configuration");
	*config = fixture_enabled("THERMALMASTER_NATIVE_FIXTURE_UNCONFIGURED") ? 0 : 1;
	return fixture_failure("get-configuration");
}

int LIBUSB_CALL libusb_set_configuration(libusb_device_handle *dev_handle, int configuration) {
	(void)dev_handle;
	(void)configuration;
	fixture_log("set-configuration");
	return fixture_failure("set-configuration");
}

int LIBUSB_CALL libusb_set_auto_detach_kernel_driver(libusb_device_handle *dev_handle, int enable) {
	(void)dev_handle;
	(void)enable;
	fixture_log("auto-detach-enable");
	return fixture_failure("auto-detach-enable");
}

int LIBUSB_CALL libusb_detach_kernel_driver(libusb_device_handle *dev_handle, int interface_number) {
	(void)dev_handle;
	fixture_log(interface_number == 0 ? "manual-detach-0" : "manual-detach-1");
	return LIBUSB_SUCCESS;
}

int LIBUSB_CALL libusb_claim_interface(libusb_device_handle *dev_handle, int interface_number) {
	(void)dev_handle;
	const char *event = interface_number == 0 ? "claim-control" : "claim-stream";
	fixture_log(event);
	return fixture_failure(event);
}

int LIBUSB_CALL libusb_set_interface_alt_setting(libusb_device_handle *dev_handle,
                                                 int interface_number, int alternate_setting) {
	(void)dev_handle;
	const char *event = interface_number == 0 ? "control-alt" :
	    (alternate_setting == 0 ? "stream-alt0" : "stream-alt1");
	fixture_log(event);
	return fixture_failure(event);
}

int LIBUSB_CALL libusb_release_interface(libusb_device_handle *dev_handle, int interface_number) {
	(void)dev_handle;
	const char *event = interface_number == 0 ? "release-control" : "release-stream";
	fixture_log(event);
	return fixture_failure(event);
}

int LIBUSB_CALL libusb_control_transfer(libusb_device_handle *dev_handle, uint8_t request_type,
                                        uint8_t request, uint16_t value, uint16_t index,
                                        unsigned char *data, uint16_t length, unsigned int timeout) {
	(void)dev_handle;
	(void)request_type;
	(void)value;
	(void)index;
	fixture_log("vendor-control");
	if (timeout != 1000) {
		fixture_log("incorrect-control-timeout");
		return LIBUSB_ERROR_INVALID_PARAM;
	}
	if (request == 0x22 && length == 1) {
		data[0] = 2;
	} else if (data != NULL && length > 0 && request != 0x20) {
		memset(data, 0, length);
	}
	return (int)length;
}

int LIBUSB_CALL libusb_bulk_transfer(libusb_device_handle *dev_handle, unsigned char endpoint,
                                     unsigned char *data, int length, int *actual_length,
                                     unsigned int timeout) {
	(void)dev_handle;
	(void)endpoint;
	(void)data;
	(void)length;
	(void)actual_length;
	(void)timeout;
	fixture_log("unexpected-synchronous-bulk-read");
	return LIBUSB_ERROR_NOT_SUPPORTED;
}

struct libusb_transfer *LIBUSB_CALL libusb_alloc_transfer(int iso_packets) {
	fixture_log("transfer-allocate");
	return calloc(1, sizeof(struct libusb_transfer) + (size_t)iso_packets * sizeof(struct libusb_iso_packet_descriptor));
}

void LIBUSB_CALL libusb_free_transfer(struct libusb_transfer *transfer) {
	fixture_log("transfer-free");
	free(transfer);
}

int LIBUSB_CALL libusb_submit_transfer(struct libusb_transfer *transfer) {
	pthread_mutex_lock(&fixture_mutex);
	if (fixture_pending != NULL) {
		pthread_mutex_unlock(&fixture_mutex);
		return LIBUSB_ERROR_BUSY;
	}
	fixture_log("transfer-submit");
	fixture_pending = transfer;
	fixture_cancelled = 0;
	fixture_blocked = fixture_enabled("THERMALMASTER_NATIVE_FIXTURE_BLOCK");
	const char *notify_fd = getenv("THERMALMASTER_NATIVE_FIXTURE_SUBMIT_FD");
	if (notify_fd != NULL) {
		if (write(atoi(notify_fd), "x", 1) != 1) {
			fixture_log("submit-notification-failed");
		}
	}
	pthread_cond_signal(&fixture_changed);
	pthread_mutex_unlock(&fixture_mutex);
	return LIBUSB_SUCCESS;
}

int LIBUSB_CALL libusb_cancel_transfer(struct libusb_transfer *transfer) {
	pthread_mutex_lock(&fixture_mutex);
	if (fixture_pending != transfer) {
		pthread_mutex_unlock(&fixture_mutex);
		return LIBUSB_ERROR_NOT_FOUND;
	}
	fixture_log("transfer-cancel");
	fixture_cancelled = 1;
	pthread_cond_signal(&fixture_changed);
	pthread_mutex_unlock(&fixture_mutex);
	return LIBUSB_SUCCESS;
}

int LIBUSB_CALL libusb_handle_events_timeout_completed(libusb_context *ctx,
                                                        struct timeval *timeout,
                                                        int *completed) {
	(void)ctx;
	(void)completed;
	struct timespec deadline;
	clock_gettime(CLOCK_REALTIME, &deadline);
	deadline.tv_sec += timeout->tv_sec;
	deadline.tv_nsec += timeout->tv_usec * 1000;
	if (deadline.tv_nsec >= 1000000000) {
		deadline.tv_sec++;
		deadline.tv_nsec -= 1000000000;
	}
	pthread_mutex_lock(&fixture_mutex);
	while (fixture_pending == NULL || (fixture_blocked && !fixture_cancelled)) {
		if (pthread_cond_timedwait(&fixture_changed, &fixture_mutex, &deadline) == ETIMEDOUT) {
			pthread_mutex_unlock(&fixture_mutex);
			return LIBUSB_SUCCESS;
		}
	}
	struct libusb_transfer *transfer = fixture_pending;
	fixture_pending = NULL;
	memset(transfer->buffer, 0, (size_t)transfer->length);
	if (fixture_cancelled) {
		transfer->status = LIBUSB_TRANSFER_CANCELLED;
		transfer->actual_length = transfer->length >= 3 ? 3 : transfer->length;
		memcpy(transfer->buffer, "abc", (size_t)transfer->actual_length);
	} else {
		transfer->status = LIBUSB_TRANSFER_COMPLETED;
		transfer->actual_length = transfer->length;
		if (transfer->length >= 2) {
			transfer->buffer[0] = 12;
			transfer->buffer[1] = 0x8c;
		}
	}
	pthread_mutex_unlock(&fixture_mutex);
	fixture_log("transfer-callback");
	transfer->callback(transfer);
	return LIBUSB_SUCCESS;
}

int LIBUSB_CALL libusb_get_string_descriptor_ascii(libusb_device_handle *dev_handle,
                                                   uint8_t desc_index, unsigned char *data,
                                                   int length) {
	(void)desc_index;
	if (dev_handle == (libusb_device_handle *)&fixture_handle_storage[0] &&
	    fixture_enabled("THERMALMASTER_NATIVE_FIXTURE_SERIAL_FIRST_FAILS")) {
		fixture_log("serial-access-denied");
		return LIBUSB_ERROR_ACCESS;
	}
	fixture_log("serial-read");
	if (fixture_failure("serial-read") != 0) {
		return LIBUSB_ERROR_IO;
	}
	static const char serial[] = "POC123";
	int serial_length = (int)(sizeof(serial) - 1);
	if (serial_length > length) {
		serial_length = length;
	}
	memcpy(data, serial, (size_t)serial_length);
	return serial_length;
}
