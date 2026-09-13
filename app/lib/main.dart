import 'package:dynamic_color/dynamic_color.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/router/app_router.dart';
import 'core/settings/settings_controller.dart';
import 'core/theme/app_theme.dart';

void main() {
  runApp(const ProviderScope(child: AtticApp()));
}

/// Root of the Attic app.
///
/// Material 3 throughout, with the device's dynamic colour scheme on
/// Android 12+ and a seeded scheme everywhere else. Theme mode follows the
/// system unless the user overrides it in Settings.
class AtticApp extends ConsumerStatefulWidget {
  const AtticApp({super.key});

  @override
  ConsumerState<AtticApp> createState() => _AtticAppState();
}

class _AtticAppState extends ConsumerState<AtticApp> {
  // The router is created once: rebuilding it would reset navigation state
  // every time the theme changes.
  late final _router = createRouter();

  @override
  Widget build(BuildContext context) {
    final settings = ref.watch(settingsProvider);

    return DynamicColorBuilder(
      builder: (ColorScheme? lightDynamic, ColorScheme? darkDynamic) {
        return MaterialApp.router(
          title: 'Attic',
          debugShowCheckedModeBanner: false,
          theme: AppTheme.light(lightDynamic?.harmonized()),
          darkTheme: AppTheme.dark(darkDynamic?.harmonized()),
          themeMode: settings.themeMode,
          routerConfig: _router,
        );
      },
    );
  }
}
