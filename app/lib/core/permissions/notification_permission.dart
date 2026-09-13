import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:permission_handler/permission_handler.dart';

/// What the platform currently says about notification permission.
enum NotificationAccess {
  /// Granted, or the platform does not gate notifications at all.
  granted,

  /// Not granted yet, and the system will show a prompt if asked.
  askable,

  /// Refused in a way the system will no longer prompt for. Only a trip to
  /// app settings changes this.
  blocked,
}

/// The platform calls this feature needs, behind an interface so the policy
/// above it can be tested without a device.
abstract class NotificationPermissionPlatform {
  /// Whether this platform gates notifications at all.
  bool get isGated;

  Future<NotificationAccess> status();
  Future<NotificationAccess> request();
  Future<void> openSettings();

  /// Whether asking again would actually put a prompt on screen.
  ///
  /// Android reports this as "should show rationale": it is true only between
  /// the first refusal and the last. Once the system has stopped prompting,
  /// a request returns denied without showing anything, and the only route
  /// left is app settings.
  Future<bool> willPrompt();
}

/// permission_handler, for real devices.
class PlatformNotificationPermission implements NotificationPermissionPlatform {
  const PlatformNotificationPermission();

  // Android 13 (API 33) introduced the runtime notification permission. iOS
  // does not need it for what Attic posts: the Now Playing controls come from
  // the media session, not from a local notification.
  @override
  bool get isGated => Platform.isAndroid;

  @override
  Future<NotificationAccess> status() async =>
      _map(await Permission.notification.status);

  @override
  Future<NotificationAccess> request() async =>
      _map(await Permission.notification.request());

  @override
  Future<void> openSettings() => openAppSettings();

  @override
  Future<bool> willPrompt() => Permission.notification.shouldShowRequestRationale;

  static NotificationAccess _map(PermissionStatus status) {
    if (status.isGranted || status.isLimited || status.isProvisional) {
      return NotificationAccess.granted;
    }
    if (status.isPermanentlyDenied || status.isRestricted) {
      return NotificationAccess.blocked;
    }
    return NotificationAccess.askable;
  }
}

/// Decides when to ask for notification permission.
///
/// Attic asks at the moment the answer first matters — when playback starts —
/// rather than on first launch. Without the permission the music still plays;
/// what is lost is the notification the lock-screen and headset controls live
/// in, which is a thing worth explaining in context and not worth nagging
/// about.
class NotificationPermissionController extends StateNotifier<NotificationAccess> {
  NotificationPermissionController(this._platform)
      : super(_platform.isGated ? NotificationAccess.askable : NotificationAccess.granted);

  final NotificationPermissionPlatform _platform;

  /// True once a prompt has been shown in this run, so a queue of ten tracks
  /// does not mean ten prompts.
  bool _askedThisRun = false;

  /// Whether the media notification can be posted.
  bool get isGranted => state == NotificationAccess.granted;

  /// Refreshes [state] from the platform without prompting.
  Future<NotificationAccess> refresh() async {
    if (!_platform.isGated) return state = NotificationAccess.granted;

    final current = await _platform.status();
    if (current != NotificationAccess.askable || !_askedThisRun) {
      return state = current;
    }
    // Already asked and still not granted: say whether asking again is worth
    // offering.
    return state = await _resolveDenial(current);
  }

  /// Asks for permission if that is still worth doing.
  ///
  /// Returns the resulting access. Never throws and never blocks playback: a
  /// refusal is a degraded media notification, not a failure.
  Future<NotificationAccess> ensure() async {
    if (!_platform.isGated) return state = NotificationAccess.granted;

    final current = await _platform.status();
    if (current != NotificationAccess.askable) return state = current;

    // The system only prompts once; asking again in the same run would do
    // nothing but delay playback.
    if (_askedThisRun) return state = current;
    _askedThisRun = true;

    final result = await _platform.request();
    if (result == NotificationAccess.granted) return state = result;

    // A request can come back denied without ever having shown a prompt —
    // the system stops asking after enough refusals, and reports that denial
    // exactly like a fresh one. Left alone, the UI would keep offering an
    // "Allow" button that silently does nothing, so settle which of the two
    // this was before reporting it.
    return state = await _resolveDenial(result);
  }

  /// Distinguishes "refused, but asking again would prompt" from "the system
  /// will not prompt any more".
  Future<NotificationAccess> _resolveDenial(NotificationAccess reported) async {
    if (reported == NotificationAccess.blocked) return reported;
    return await _platform.willPrompt()
        ? NotificationAccess.askable
        : NotificationAccess.blocked;
  }

  /// Opens the system settings page, the only route out of [blocked].
  Future<void> openSettings() => _platform.openSettings();
}

final notificationPermissionPlatformProvider =
    Provider<NotificationPermissionPlatform>((ref) => const PlatformNotificationPermission());

final notificationPermissionProvider =
    StateNotifierProvider<NotificationPermissionController, NotificationAccess>((ref) {
  return NotificationPermissionController(ref.watch(notificationPermissionPlatformProvider));
});
