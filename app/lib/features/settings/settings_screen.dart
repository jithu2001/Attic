import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/flags.dart';
import '../../core/layout/window_size.dart';
import '../../core/settings/settings_controller.dart';

/// Settings is live from the first phase: it owns the theme override and the
/// server connection entry point, so the app shell is genuinely usable.
class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settings = ref.watch(settingsProvider);
    final text = Theme.of(context).textTheme;
    final colors = Theme.of(context).colorScheme;
    final margin = WindowSizeClass.of(context).screenMargin;

    return Scaffold(
      appBar: AppBar(title: const Text('Settings')),
      body: ListView(
        padding: EdgeInsets.symmetric(horizontal: margin, vertical: 8),
        children: <Widget>[
          const _SectionHeader(label: 'Appearance'),
          Card.outlined(
            child: Column(
              children: <Widget>[
                Padding(
                  padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
                  child: Row(
                    children: <Widget>[
                      Icon(Icons.brightness_6_outlined,
                          color: colors.onSurfaceVariant),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Text('Theme', style: text.titleMedium),
                      ),
                    ],
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                  child: SegmentedButton<ThemeMode>(
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
                ),
              ],
            ),
          ),
          const SizedBox(height: 24),
          const _SectionHeader(label: 'Server'),
          Card.outlined(
            child: Column(
              children: <Widget>[
                ListTile(
                  leading: const Icon(Icons.dns_outlined),
                  title: const Text('Server address'),
                  subtitle: const Text(
                    Flags.auth
                        ? 'Not connected'
                        : 'Connecting to a server arrives with sign-in',
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  enabled: Flags.auth,
                  onTap: Flags.auth ? () => context.go('/connect') : null,
                ),
                const Divider(height: 1),
                ListTile(
                  leading: const Icon(Icons.info_outline),
                  title: const Text('About Attic'),
                  subtitle: const Text('Version 0.1.0 — skeleton'),
                  onTap: () => showAboutDialog(
                    context: context,
                    applicationName: 'Attic',
                    applicationVersion: '0.1.0',
                    children: const <Widget>[
                      Text(
                        'Self-hosted photos, music and video, reachable only '
                        'over your Tailscale network.',
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
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
