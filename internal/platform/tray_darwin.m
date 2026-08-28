#import <Cocoa/Cocoa.h>

extern void fiberPulseTrayAction(int identifier);

@interface FiberPulseTrayDelegate : NSObject
@property(nonatomic, strong) NSStatusItem *statusItem;
@end

@implementation FiberPulseTrayDelegate

- (void)menuAction:(id)sender {
    fiberPulseTrayAction((int)[sender tag]);
}

- (void)stopApplication {
    [NSApp stop:nil];
    NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                        location:NSZeroPoint
                                   modifierFlags:0
                                       timestamp:0
                                    windowNumber:0
                                         context:nil
                                         subtype:0
                                           data1:0
                                           data2:0];
    [NSApp postEvent:event atStart:NO];
}

@end

static FiberPulseTrayDelegate *fiberPulseDelegate;

static void addMenuItem(NSMenu *menu, NSString *title, NSInteger tag) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title
                                                 action:@selector(menuAction:)
                                          keyEquivalent:@""];
    item.target = fiberPulseDelegate;
    item.tag = tag;
    [menu addItem:item];
}

void FiberPulseTrayRun(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        fiberPulseDelegate = [[FiberPulseTrayDelegate alloc] init];
        fiberPulseDelegate.statusItem = [[NSStatusBar systemStatusBar]
            statusItemWithLength:NSVariableStatusItemLength];
        fiberPulseDelegate.statusItem.button.title = @"FP";
        fiberPulseDelegate.statusItem.button.toolTip = @"FiberPulse — measured Internet performance";

        NSMenu *menu = [[NSMenu alloc] initWithTitle:@"FiberPulse"];
        addMenuItem(menu, @"Open FiberPulse", 1);
        addMenuItem(menu, @"Run manual test", 2);
        addMenuItem(menu, @"Pause / resume", 3);
        addMenuItem(menu, @"Open reports", 4);
        addMenuItem(menu, @"Check for update", 5);
        [menu addItem:[NSMenuItem separatorItem]];
        addMenuItem(menu, @"Quit completely", 6);
        fiberPulseDelegate.statusItem.menu = menu;

        [NSApp run];
        [[NSStatusBar systemStatusBar] removeStatusItem:fiberPulseDelegate.statusItem];
        fiberPulseDelegate.statusItem = nil;
        fiberPulseDelegate = nil;
    }
}

void FiberPulseTrayStop(void) {
    FiberPulseTrayDelegate *delegate = fiberPulseDelegate;
    if (delegate != nil) {
        [delegate performSelectorOnMainThread:@selector(stopApplication)
                                  withObject:nil
                               waitUntilDone:NO];
    }
}
