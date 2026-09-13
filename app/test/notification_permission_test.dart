import 'package:attic/core/permissions/notification_permission.dart';
import 'package:flutter_test/flutter_test.dart';

/// A scriptable stand-in for permission_handler.
class FakePlatform implements NotificationPermissionPlatform {
  FakePlatform({
    this.isGated = true,
    NotificationAccess initial = NotificationAccess.askable,
    this.answer,
    this.promptsAgain = true,
  }) : _current = initial;

  @override
  final bool isGated;

  NotificationAccess _current;

  /// What the system prompt returns. Defaults to granting.
  final NotificationAccess? answer;

  /// Whether the system would put a prompt on screen if asked again.
  final bool promptsAgain;

  int requestCount = 0;
  int statusCount = 0;
  int openSettingsCount = 0;

  @override
  Future<NotificationAccess> status() async {
    statusCount++;
    return _current;
  }

  @override
  Future<NotificationAccess> request() async {
    requestCount++;
    return _current = answer ?? NotificationAccess.granted;
  }

  @override
  Future<void> openSettings() async => openSettingsCount++;

  @override
  Future<bool> willPrompt() async => promptsAgain;
}

void main() {
  group('NotificationPermissionController', () {
    test('does not prompt on a platform that does not gate notifications', () async {
      final platform = FakePlatform(isGated: false);
      final controller = NotificationPermissionController(platform);

      expect(controller.isGranted, isTrue);
      expect(await controller.ensure(), NotificationAccess.granted);
      expect(platform.requestCount, 0);
      expect(platform.statusCount, 0);
    });

    test('prompts once and records the grant', () async {
      final platform = FakePlatform();
      final controller = NotificationPermissionController(platform);

      expect(await controller.ensure(), NotificationAccess.granted);
      expect(platform.requestCount, 1);
      expect(controller.isGranted, isTrue);
    });

    test('does not prompt again once permission is granted', () async {
      final platform = FakePlatform(initial: NotificationAccess.granted);
      final controller = NotificationPermissionController(platform);

      await controller.ensure();
      await controller.ensure();

      expect(platform.requestCount, 0, reason: 'already granted; nothing to ask');
    });

    test('asks at most once per run, however many tracks are played', () async {
      // Every playQueue call goes through ensure(); the system only prompts
      // once, so asking again would just delay playback.
      final platform = FakePlatform(answer: NotificationAccess.askable);
      final controller = NotificationPermissionController(platform);

      await controller.ensure();
      await controller.ensure();
      await controller.ensure();

      expect(platform.requestCount, 1);
    });

    test('never prompts once the system has stopped prompting', () async {
      final platform = FakePlatform(initial: NotificationAccess.blocked);
      final controller = NotificationPermissionController(platform);

      expect(await controller.ensure(), NotificationAccess.blocked);
      expect(platform.requestCount, 0,
          reason: 'a blocked permission can only be changed in settings');
      expect(controller.isGranted, isFalse);
    });

    test('refresh picks up a grant made outside the app', () async {
      final platform = FakePlatform(initial: NotificationAccess.blocked);
      final controller = NotificationPermissionController(platform);
      await controller.refresh();
      expect(controller.state, NotificationAccess.blocked);

      // The user went to system settings and switched it on.
      platform._current = NotificationAccess.granted;

      expect(await controller.refresh(), NotificationAccess.granted);
      expect(controller.isGranted, isTrue);
    });

    test('refresh does not prompt', () async {
      final platform = FakePlatform();
      final controller = NotificationPermissionController(platform);

      await controller.refresh();

      expect(platform.requestCount, 0);
      expect(platform.statusCount, 1);
    });

    test('openSettings reaches the platform', () async {
      final platform = FakePlatform(initial: NotificationAccess.blocked);
      final controller = NotificationPermissionController(platform);

      await controller.openSettings();

      expect(platform.openSettingsCount, 1);
    });

    test('a refusal that can be retried stays askable', () async {
      final platform = FakePlatform(
        answer: NotificationAccess.askable,
        promptsAgain: true,
      );
      final controller = NotificationPermissionController(platform);

      expect(await controller.ensure(), NotificationAccess.askable);
    });

    test('a denial the system will not re-prompt for becomes blocked', () async {
      // The dead end this guards: the platform can return a plain "denied"
      // without ever showing a prompt, and an "Allow" button that silently
      // does nothing is worse than one that sends you to settings.
      final platform = FakePlatform(
        answer: NotificationAccess.askable,
        promptsAgain: false,
      );
      final controller = NotificationPermissionController(platform);

      expect(await controller.ensure(), NotificationAccess.blocked);
    });

    test('refresh after a silent denial reports blocked, not askable', () async {
      final platform = FakePlatform(
        answer: NotificationAccess.askable,
        promptsAgain: false,
      );
      final controller = NotificationPermissionController(platform);

      await controller.ensure();
      expect(await controller.refresh(), NotificationAccess.blocked);
    });

    test('refresh before anything was asked does not guess', () async {
      final platform = FakePlatform(promptsAgain: false);
      final controller = NotificationPermissionController(platform);

      // Nothing has been asked yet, so a first prompt is still coming.
      expect(await controller.refresh(), NotificationAccess.askable);
    });

    test('a refusal is reported, not thrown', () async {
      // Playback must continue without the notification, so the controller
      // reports the state rather than failing.
      final platform = FakePlatform(answer: NotificationAccess.blocked);
      final controller = NotificationPermissionController(platform);

      expect(await controller.ensure(), NotificationAccess.blocked);
      expect(controller.isGranted, isFalse);
    });
  });
}
