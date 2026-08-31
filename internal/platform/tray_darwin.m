#import <Cocoa/Cocoa.h>

extern void fiberPulseTrayAction(int identifier);

@interface FiberPulseTrayDelegate : NSObject
@property(nonatomic, strong) NSStatusItem *statusItem;
@property(nonatomic, strong) NSMenuItem *versionItem;
@property(nonatomic, strong) NSMenuItem *pauseItem;
@property(nonatomic, strong) NSMenuItem *checkItem;
@property(nonatomic, strong) NSMenuItem *installItem;
@property(nonatomic, strong) NSImage *normalIcon;
@property(nonatomic, strong) NSImage *notificationIcon;
@end

@implementation FiberPulseTrayDelegate
- (void)menuAction:(id)sender { fiberPulseTrayAction((int)[sender tag]); }
- (void)stopApplication {
    [NSApp stop:nil];
    NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                        location:NSZeroPoint modifierFlags:0 timestamp:0
                                    windowNumber:0 context:nil subtype:0 data1:0 data2:0];
    [NSApp postEvent:event atStart:NO];
}
@end

static FiberPulseTrayDelegate *fiberPulseDelegate;
static NSString *fiberPulseVersion = @"";
static NSString *fiberPulseUpdateStatus = @"idle";
static NSString *fiberPulseAvailableVersion = @"";
static NSString *fiberPulseUpdateError = @"";
static BOOL fiberPulsePaused = NO;

static NSImage *symbol(NSString *name) {
    if (@available(macOS 11.0, *)) {
        NSImage *image = [NSImage imageWithSystemSymbolName:name accessibilityDescription:nil];
        image.size = NSMakeSize(16, 16);
        return image;
    }
    return nil;
}

static NSImage *loadBrandIcon(void) {
    NSString *path = [[NSBundle mainBundle] pathForResource:@"FiberPulse" ofType:@"icns"];
    NSImage *image = path ? [[NSImage alloc] initWithContentsOfFile:path] : nil;
    if (image == nil) image = symbol(@"waveform.path.ecg");
    image.size = NSMakeSize(19, 19);
    image.template = NO;
    return image;
}

static NSImage *badgedIcon(NSImage *base) {
    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(19, 19)];
    [image lockFocus];
    [base drawInRect:NSMakeRect(0, 0, 19, 19)];
    [[NSColor colorWithRed:1 green:0.22 blue:0.29 alpha:1] setFill];
    NSBezierPath *badge = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(12, 12, 7, 7)];
    [badge fill];
    [[NSColor whiteColor] setStroke];
    badge.lineWidth = 1.2;
    [badge stroke];
    [image unlockFocus];
    image.template = NO;
    return image;
}

static NSMenuItem *addMenuItem(NSMenu *menu, NSString *title, NSInteger tag, NSString *symbolName) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:@selector(menuAction:) keyEquivalent:@""];
    item.target = fiberPulseDelegate;
    item.tag = tag;
    if (symbolName) item.image = symbol(symbolName);
    [menu addItem:item];
    return item;
}

static void applyState(void) {
    if (fiberPulseDelegate == nil) return;
    NSString *shownVersion = fiberPulseVersion.length ? fiberPulseVersion : @"Desktop";
    fiberPulseDelegate.versionItem.title = [NSString stringWithFormat:@"FiberPulse %@  ·  Connection monitor", shownVersion];
    fiberPulseDelegate.pauseItem.title = fiberPulsePaused ? @"Resume automatic monitoring" : @"Pause automatic monitoring";
    fiberPulseDelegate.pauseItem.image = symbol(fiberPulsePaused ? @"play.fill" : @"pause.fill");
    BOOL updateAvailable = [fiberPulseUpdateStatus isEqualToString:@"available"] && fiberPulseAvailableVersion.length > 0;
    fiberPulseDelegate.installItem.hidden = !updateAvailable;
    if (updateAvailable) {
        fiberPulseDelegate.checkItem.title = [NSString stringWithFormat:@"Update ready  ·  %@ → %@", shownVersion, fiberPulseAvailableVersion];
        fiberPulseDelegate.checkItem.image = symbol(@"bell.badge.fill");
        fiberPulseDelegate.installItem.title = [NSString stringWithFormat:@"Install FiberPulse %@…", fiberPulseAvailableVersion];
        fiberPulseDelegate.statusItem.button.image = fiberPulseDelegate.notificationIcon;
        fiberPulseDelegate.statusItem.button.toolTip = [NSString stringWithFormat:@"FiberPulse %@ — update %@ available", shownVersion, fiberPulseAvailableVersion];
    } else {
        fiberPulseDelegate.checkItem.title = [fiberPulseUpdateStatus isEqualToString:@"checking"] ? @"Checking for updates…" : @"Check for updates…";
        fiberPulseDelegate.checkItem.image = symbol(@"arrow.triangle.2.circlepath");
        fiberPulseDelegate.statusItem.button.image = fiberPulseDelegate.normalIcon;
        fiberPulseDelegate.statusItem.button.toolTip = [NSString stringWithFormat:@"FiberPulse %@ — measured Internet performance", shownVersion];
    }
}

void FiberPulseTrayRun(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        fiberPulseDelegate = [[FiberPulseTrayDelegate alloc] init];
        fiberPulseDelegate.normalIcon = loadBrandIcon();
        fiberPulseDelegate.notificationIcon = badgedIcon(fiberPulseDelegate.normalIcon);
        fiberPulseDelegate.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
        fiberPulseDelegate.statusItem.button.image = fiberPulseDelegate.normalIcon;
        fiberPulseDelegate.statusItem.button.imagePosition = NSImageOnly;

        NSMenu *menu = [[NSMenu alloc] initWithTitle:@"FiberPulse"];
        menu.autoenablesItems = NO;
        fiberPulseDelegate.versionItem = [[NSMenuItem alloc] initWithTitle:@"FiberPulse" action:nil keyEquivalent:@""];
        fiberPulseDelegate.versionItem.enabled = NO;
        [menu addItem:fiberPulseDelegate.versionItem];
        [menu addItem:[NSMenuItem separatorItem]];
        addMenuItem(menu, @"Open dashboard", 1, @"gauge.with.dots.needle.67percent");
        addMenuItem(menu, @"Run a manual connection test", 2, @"waveform.path.ecg");
        fiberPulseDelegate.pauseItem = addMenuItem(menu, @"Pause automatic monitoring", 3, @"pause.fill");
        addMenuItem(menu, @"History and reports", 4, @"doc.text.magnifyingglass");
        [menu addItem:[NSMenuItem separatorItem]];
        fiberPulseDelegate.checkItem = addMenuItem(menu, @"Check for updates…", 5, @"arrow.triangle.2.circlepath");
        fiberPulseDelegate.installItem = addMenuItem(menu, @"Install update…", 6, @"arrow.down.circle.fill");
        fiberPulseDelegate.installItem.hidden = YES;
        [menu addItem:[NSMenuItem separatorItem]];
        addMenuItem(menu, @"Quit FiberPulse completely", 7, @"power");
        fiberPulseDelegate.statusItem.menu = menu;
        applyState();

        [NSApp run];
        [[NSStatusBar systemStatusBar] removeStatusItem:fiberPulseDelegate.statusItem];
        fiberPulseDelegate = nil;
    }
}

void FiberPulseTraySetState(const char *version, int paused, const char *status, const char *available, const char *error) {
    NSString *versionCopy = [NSString stringWithUTF8String:version ?: ""];
    NSString *statusCopy = [NSString stringWithUTF8String:status ?: ""];
    NSString *availableCopy = [NSString stringWithUTF8String:available ?: ""];
    NSString *errorCopy = [NSString stringWithUTF8String:error ?: ""];
    dispatch_async(dispatch_get_main_queue(), ^{
        fiberPulseVersion = versionCopy;
        fiberPulsePaused = paused == 1;
        fiberPulseUpdateStatus = statusCopy;
        fiberPulseAvailableVersion = availableCopy;
        fiberPulseUpdateError = errorCopy;
        applyState();
    });
}

int FiberPulseTrayPresentUpdate(const char *version, const char *status, const char *available, const char *error) {
    __block NSInteger response = NSAlertSecondButtonReturn;
    void (^showAlert)(void) = ^{
        NSString *current = [NSString stringWithUTF8String:version ?: ""];
        NSString *state = [NSString stringWithUTF8String:status ?: ""];
        NSString *next = [NSString stringWithUTF8String:available ?: ""];
        NSString *failure = [NSString stringWithUTF8String:error ?: ""];
        NSAlert *alert = [[NSAlert alloc] init];
        alert.icon = fiberPulseDelegate.normalIcon ?: loadBrandIcon();
        if ([state isEqualToString:@"available"] && next.length > 0) {
            alert.messageText = @"A FiberPulse update is available";
            alert.informativeText = [NSString stringWithFormat:@"Installed version: %@\nAvailable version: %@\n\nThe signed update will be downloaded, verified and installed only if you approve it.", current, next];
            [alert addButtonWithTitle:[NSString stringWithFormat:@"Install %@", next]];
            [alert addButtonWithTitle:@"Later"];
        } else if ([state isEqualToString:@"up_to_date"]) {
            alert.messageText = @"FiberPulse is up to date";
            alert.informativeText = [NSString stringWithFormat:@"Version %@ is the newest verified release available for this channel.", current];
            [alert addButtonWithTitle:@"OK"];
        } else {
            alert.messageText = @"Update check could not complete";
            alert.informativeText = failure.length ? failure : @"No signed update channel is configured for this build.";
            [alert addButtonWithTitle:@"OK"];
        }
        [NSApp activateIgnoringOtherApps:YES];
        response = [alert runModal];
    };
    if ([NSThread isMainThread]) showAlert(); else dispatch_sync(dispatch_get_main_queue(), showAlert);
    BOOL canInstall = [[NSString stringWithUTF8String:status ?: ""] isEqualToString:@"available"];
    return response == NSAlertFirstButtonReturn && canInstall ? 1 : 0;
}

void FiberPulseTrayStop(void) {
    FiberPulseTrayDelegate *delegate = fiberPulseDelegate;
    if (delegate != nil) [delegate performSelectorOnMainThread:@selector(stopApplication) withObject:nil waitUntilDone:NO];
}
