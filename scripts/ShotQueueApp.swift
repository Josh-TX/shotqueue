import Cocoa
import Foundation

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var childProcess: Process?
    private var monitorTimer: Timer?

    private var pidFileURL: URL {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first ?? URL(fileURLWithPath: NSHomeDirectory())
        let dir = support.appendingPathComponent("ShotQueue", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("shotqueue.pid")
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)

        if let pidData = try? Data(contentsOf: pidFileURL),
           let pidString = String(data: pidData, encoding: .utf8),
           let pid = pid_t(pidString.trimmingCharacters(in: .whitespacesAndNewlines)),
           kill(pid, 0) == 0 {
            NSApp.terminate(nil)
            return
        }

        let backendURL = Bundle.main.bundleURL
            .appendingPathComponent("Contents")
            .appendingPathComponent("Resources")
            .appendingPathComponent("shotqueue-bin")

        let process = Process()
        process.executableURL = backendURL
        process.arguments = ["--port", "8080"]
        process.standardOutput = FileHandle.standardOutput
        process.standardError = FileHandle.standardError

        do {
            try process.run()
            childProcess = process
            let pidText = String(process.processIdentifier)
            try pidText.write(to: pidFileURL, atomically: true, encoding: .utf8)
        } catch {
            NSApp.terminate(nil)
            return
        }

        monitorTimer = Timer.scheduledTimer(withTimeInterval: 1.0, repeats: true) { [weak self] _ in
            guard let self else { return }
            if let child = self.childProcess, !child.isRunning {
                self.cleanupPID()
                NSApp.terminate(nil)
            }
        }
    }

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        cleanupPID()
        if let child = childProcess, child.isRunning {
            child.terminate()
        }
        return .terminateNow
    }

    private func cleanupPID() {
        try? FileManager.default.removeItem(at: pidFileURL)
        monitorTimer?.invalidate()
        monitorTimer = nil
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
NSApplicationMain(CommandLine.argc, CommandLine.unsafeArgv)
