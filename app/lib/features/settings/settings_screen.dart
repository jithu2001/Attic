import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_error.dart';
import '../../core/layout/window_size.dart';
import '../../core/permissions/notification_permission.dart';
import '../../core/providers.dart';
import '../../core/settings/settings_controller.dart';
import '../auth/auth_controller.dart';

/// Devices signed in to this account, read-only for now.
final devicesProvider = FutureProvider.autoDispose<List<Map<String, dynamic>>>((ref) async {
  final json = await ref.watch(apiClientProvider).getJson('/me/devices');
  return (json['devices'] as List<dynamic>? ?? const [])
      .cast<Map<String, dynamic>>();
});

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settings = ref.watch(settingsProvider);
    final auth = ref.watch(authControllerProvider);
    final margin = WindowSizeClass.of(context).screenMargin;

    return Scaffold(
      appBar: AppBar(title: const Text('Settings')),
      body: ListView(
        padding: EdgeInsets.symmetric(horizontal: margin, vertical: 8),
        children: <Widget>[
          const _SectionHeader(label: 'Appearance'),
          Card.outlined(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text('Theme', style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 12),
                  SegmentedButton<ThemeMode>(
                    segments: const <ButtonSegment<ThemeMode>>[
                      ButtonSegment<ThemeMode>(
                        value: ThemeMode.system,
                        label: Text('System'),
                        icon: Icon(Icons.phone_android),
                      ),
                      ButtonSegment<ThemeMode>(
                        value: ThemeMode.light,
                        label: Text('Light'),
                        icon: Icon(Icons.light_mode_outlined),
                      ),
                      ButtonSegment<ThemeMode>(
                        value: ThemeMode.dark,
                        label: Text('Dark'),
                        icon: Icon(Icons.dark_mode_outlined),
                      ),
                    ],
                    selected: <ThemeMode>{settings.themeMode},
                    onSelectionChanged: (selection) => ref
                        .read(settingsProvider.notifier)
                        .setThemeMode(selection.first),
                  ),
                ],
              ),
            ),
          ),
          const NotificationPermissionSection(),
          const SizedBox(height: 24),
          const _SectionHeader(label: 'Account'),
          Card.outlined(
            child: Column(
              children: <Widget>[
                ListTile(
                  leading: const Icon(Icons.person_outline),
                  title: Text(auth.account?.username ?? 'Signed in'),
                  subtitle: Text(_roleLabel(auth.account?.role)),
                ),
                const Divider(height: 1),
                ListTile(
                  leading: const Icon(Icons.dns_outlined),
                  title: const Text('Server'),
                  subtitle: Text(auth.serverUrl ?? 'Not connected'),
                ),
              ],
            ),
          ),
          const SizedBox(height: 24),
          const _SectionHeader(label: 'Devices'),
          const _DeviceList(),
          const SizedBox(height: 24),
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 8),
            child: FilledButton.tonalIcon(
              onPressed: () => _confirmSignOut(context, ref),
              icon: const Icon(Icons.logout),
              label: const Text('Sign out'),
            ),
          ),
          const SizedBox(height: 8),
          Center(
            child: TextButton(
              onPressed: () => showAboutDialog(
                context: context,
                applicationName: 'Attic',
                applicationVersion: '0.1.0',
                children: const <Widget>[
                  Text(
                    'Self-hosted photos, music and video, reachable only over '
                    'your Tailscale network.',
                  ),
                ],
              ),
              child: const Text('About Attic'),
            ),
          ),
          const SizedBox(height: 96),
        ],
      ),
    );
  }

  static String _roleLabel(String? role) => switch (role) {
        'admin' => 'Administrator',
        'kid' => 'Kid account',
        'member' => 'Member',
        _ => '',
      };

  Future<void> _confirmSignOut(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sign out?'),
        content: const Text(
          'This device will be revoked on the server. Your library is not '
          'affected.',
        ),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('Sign out'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    await ref.read(authControllerProvider.notifier).signOut();
  }
}

/// The Playback section: its heading and the permission card together.
///
/// Both or neither — a section heading with nothing under it reads as a
/// rendering bug, so the heading lives with the only thing that fills it.
class NotificationPermissionSection extends ConsumerWidget {
  const NotificationPermissionSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // The card still mounts when permission is granted, because it is what
    // watches for the permission being revoked while the app is open.
    final visible = ref.watch(notificationPermissionProvider) != NotificationAccess.granted;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        if (visible) ...<Widget>[
          const SizedBox(height: 24),
          const _SectionHeader(label: 'Playback'),
        ],
        const NotificationPermissionCard(),
      ],
    );
  }
}

/// Explains, and offers to fix, a missing notification permission.
///
/// It shows nothing at all when permission is granted: a settings screen full
/// of resolved warnings trains people to ignore them.
class NotificationPermissionCard extends ConsumerStatefulWidget {
  const NotificationPermissionCard({super.key});

  @override
  ConsumerState<NotificationPermissionCard> createState() =>
      _NotificationPermissionCardState();
}

class _NotificationPermissionCardState
    extends ConsumerState<NotificationPermissionCard> with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(notificationPermissionProvider.notifier).refresh();
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Coming back from the system settings page is the one moment this can
    // have changed without the app being told.
    if (state == AppLifecycleState.resumed) {
      ref.read(notificationPermissionProvider.notifier).refresh();
    }
  }

  @override
  Widget build(BuildContext context) {
    final access = ref.watch(notificationPermissionProvider);
    if (access == NotificationAccess.granted) return const SizedBox.shrink();

    final colors = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final blocked = access == NotificationAccess.blocked;

    return Card.outlined(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Icon(Icons.notifications_off_outlined, color: colors.onSurfaceVariant),
                const SizedBox(width: 16),
                Expanded(
                  child: Text('Lock-screen controls', style: text.titleMedium),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              blocked
                  ? 'Notifications are turned off for Attic, so the player '
                      'controls will not appear on your lock screen. You can '
                      'turn them back on in system settings.'
                  : 'Attic needs permission to post a notification. That '
                      'notification is where the lock-screen and headset '
                      'controls live. Music plays either way.',
              style: text.bodyMedium?.copyWith(color: colors.onSurfaceVariant),
            ),
            const SizedBox(height: 16),
            Align(
              alignment: Alignment.centerLeft,
              child: FilledButton.tonal(
                onPressed: () async {
                  final controller = ref.read(notificationPermissionProvider.notifier);
                  if (blocked) {
                    await controller.openSettings();
                  } else {
                    await controller.ensure();
                  }
                },
                child: Text(blocked ? 'Open settings' : 'Allow notifications'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _DeviceList extends ConsumerWidget {
  const _DeviceList();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final devices = ref.watch(devicesProvider);

    return Card.outlined(
      child: devices.when(
        loading: () => const Padding(
          padding: EdgeInsets.all(24),
          child: Center(child: CircularProgressIndicator()),
        ),
        error: (error, _) => ListTile(
          leading: const Icon(Icons.error_outline),
          title: Text(error is ApiError ? error.message : 'Could not load devices.'),
          trailing: IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: () => ref.invalidate(devicesProvider),
          ),
        ),
        data: (items) => Column(
          children: <Widget>[
            for (var i = 0; i < items.length; i++) ...<Widget>[
              if (i > 0) const Divider(height: 1),
              ListTile(
                leading: const Icon(Icons.devices_outlined),
                title: Text(items[i]['name']?.toString() ?? 'Device'),
                subtitle: Text('Last seen ${_relative(items[i]['last_seen'])}'),
              ),
            ],
          ],
        ),
      ),
    );
  }

  /// "3 hours ago" reads better than a timestamp for a device list.
  static String _relative(Object? iso) {
    final parsed = DateTime.tryParse(iso?.toString() ?? '');
    if (parsed == null) return 'recently';

    final delta = DateTime.now().toUtc().difference(parsed.toUtc());
    if (delta.inMinutes < 2) return 'just now';
    if (delta.inHours < 1) return '${delta.inMinutes} minutes ago';
    if (delta.inDays < 1) return '${delta.inHours} hours ago';
    if (delta.inDays < 30) return '${delta.inDays} days ago';
    return 'a while ago';
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 8, 4, 8),
      child: Text(
        label,
        style: Theme.of(context).textTheme.titleSmall?.copyWith(
              color: Theme.of(context).colorScheme.primary,
            ),
      ),
    );
  }
}
