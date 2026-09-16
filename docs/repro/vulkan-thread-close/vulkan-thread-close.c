// Windows x64 Vulkan teardown reproducer. See README.md for build/run commands.
// No Go runtime, renderer, submissions, extensions, layers, or custom allocator.
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#define VK_NO_PROTOTYPES
#include <vulkan/vulkan_core.h>

typedef struct {
    HANDLE start, done;
    void (*fn)(void);
    DWORD id;
} Worker;

static HMODULE vulkan;
static VkInstance instance;
static VkDevice device;
static const char *level = "device";

static void fail(const char *op, long result) {
    fprintf(stderr, "%s: %ld\n", op, result);
    ExitProcess(2);
}
static FARPROC sym(const char *name) {
    FARPROC p = GetProcAddress(vulkan, name);
    if (!p) fail(name, GetLastError());
    return p;
}
static void check(VkResult result, const char *op) {
    if (result != VK_SUCCESS) fail(op, result);
}
static void *allocate(size_t count, size_t size) {
    void *p = calloc(count, size);
    if (!p) fail("calloc", -1);
    return p;
}
static void createInstance(void) {
    VkApplicationInfo app = {0};
    app.sType = VK_STRUCTURE_TYPE_APPLICATION_INFO;
    app.pApplicationName = "thread-close-repro";
    app.pEngineName = "none";
    app.apiVersion = VK_API_VERSION_1_0;
    VkInstanceCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO;
    ci.pApplicationInfo = &app;
    check(((PFN_vkCreateInstance)sym("vkCreateInstance"))(&ci, NULL, &instance),
          "vkCreateInstance");
}
static void createDevice(void) {
    if (!strcmp(level, "instance")) return;
    PFN_vkEnumeratePhysicalDevices enumerate =
        (PFN_vkEnumeratePhysicalDevices)sym("vkEnumeratePhysicalDevices");
    uint32_t count = 0;
    check(enumerate(instance, &count, NULL), "enumerate count");
    if (!count) fail("no physical device", -1);
    VkPhysicalDevice *physical = allocate(count, sizeof(*physical));
    check(enumerate(instance, &count, physical), "enumerate devices");
    if (!strcmp(level, "enumerate")) {
        free(physical);
        return;
    }
    PFN_vkGetPhysicalDeviceQueueFamilyProperties queues =
        (PFN_vkGetPhysicalDeviceQueueFamilyProperties)sym("vkGetPhysicalDeviceQueueFamilyProperties");
    uint32_t familyCount = 0;
    queues(physical[0], &familyCount, NULL);
    if (!familyCount) fail("no queue families", -1);
    VkQueueFamilyProperties *families = allocate(familyCount, sizeof(*families));
    queues(physical[0], &familyCount, families);
    uint32_t family = 0;
    while (family < familyCount && !(families[family].queueFlags & VK_QUEUE_GRAPHICS_BIT)) family++;
    if (family == familyCount) fail("no graphics queue", -1);
    float priority = 1.0f;
    VkDeviceQueueCreateInfo queue = {0};
    queue.sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
    queue.queueFamilyIndex = family;
    queue.queueCount = 1;
    queue.pQueuePriorities = &priority;
    VkDeviceCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO;
    ci.queueCreateInfoCount = 1;
    ci.pQueueCreateInfos = &queue;
    check(((PFN_vkCreateDevice)sym("vkCreateDevice"))(physical[0], &ci, NULL, &device),
          "vkCreateDevice");
    free(families);
    free(physical);
}
static void destroyDevice(void) {
    if (!device) return;
    check(((PFN_vkDeviceWaitIdle)sym("vkDeviceWaitIdle"))(device), "vkDeviceWaitIdle");
    ((PFN_vkDestroyDevice)sym("vkDestroyDevice"))(device, NULL);
    device = VK_NULL_HANDLE;
}
static void destroyInstance(void) {
    ((PFN_vkDestroyInstance)sym("vkDestroyInstance"))(instance, NULL);
    instance = VK_NULL_HANDLE;
}
static void signal(HANDLE event) {
    if (!SetEvent(event)) fail("SetEvent", GetLastError());
}
static void wait(HANDLE event) {
    if (WaitForSingleObject(event, INFINITE) != WAIT_OBJECT_0)
        fail("WaitForSingleObject", GetLastError());
}
static DWORD WINAPI workerMain(void *arg) {
    Worker *w = arg;
    w->id = GetCurrentThreadId();
    signal(w->done);
    for (;;) {
        wait(w->start);
        w->fn();
        signal(w->done);
    }
    return 0;
}
static void initWorker(Worker *w) {
    w->start = CreateEvent(NULL, FALSE, FALSE, NULL);
    w->done = CreateEvent(NULL, FALSE, FALSE, NULL);
    if (!w->start || !w->done) fail("CreateEvent", GetLastError());
    HANDLE thread = CreateThread(NULL, 0, workerMain, w, 0, NULL);
    if (!thread) fail("CreateThread", GetLastError());
    CloseHandle(thread);
    wait(w->done);
}
static void run(Worker *w, const char *name, void (*fn)(void)) {
    printf("begin %s thread=%lu\n", name, (unsigned long)w->id);
    fflush(stdout);
    w->fn = fn;
    signal(w->start);
    wait(w->done);
    printf("end %s\n", name);
    fflush(stdout);
}
int main(int argc, char **argv) {
    const char *mode = argc > 1 ? argv[1] : "AAAB";
    if (argc > 2) level = argv[2];
    if (argc > 3 || strlen(mode) != 4 || strspn(mode, "AB") != 4 ||
        (strcmp(level, "device") && strcmp(level, "instance") && strcmp(level, "enumerate"))) {
        fprintf(stderr, "usage: %s AAAA|AAAB|... [device|instance|enumerate]\n", argv[0]);
        return 2;
    }
    char path[MAX_PATH];
    UINT length = GetSystemDirectoryA(path, MAX_PATH);
    if (!length || length + sizeof("\\vulkan-1.dll") > MAX_PATH)
        fail("GetSystemDirectoryA", -1);
    strcat(path, "\\vulkan-1.dll");
    vulkan = LoadLibraryA(path);
    if (!vulkan) fail("LoadLibraryA(vulkan-1.dll)", GetLastError());
    Worker a = {0}, b = {0};
    initWorker(&a);
    initWorker(&b);
    Worker *workers[4];
    for (int i = 0; i < 4; i++) workers[i] = mode[i] == 'B' ? &b : &a;
    printf("mode=%s level=%s A=%lu B=%lu\n", mode, level,
           (unsigned long)a.id, (unsigned long)b.id);
    for (int cycle = 1; cycle <= 2; cycle++) {
        printf("cycle=%d\n", cycle);
        run(workers[0], "createInstance", createInstance);
        run(workers[1], "createDevice", createDevice);
        run(workers[2], "destroyDevice", destroyDevice);
        run(workers[3], "destroyInstance", destroyInstance);
    }
    puts("SUCCESS");
    fflush(stdout);
    // Workers outlive all explicit teardown calls; returning ends the process.
    // Keep the Vulkan loader loaded, just as in the original reproducer.
    return 0;
}
