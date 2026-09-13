import 'package:dynamic_color/dynamic_color.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/router/app_router.dart';
import 'core/settings/settings_controller.dart';
import 'core/theme/app_theme.dart';
import 'features/auth/auth_controller.dart';
import 'features/music/player/audio_handler.dart';
import 'features/music/player/player_controller.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // The background audio service has to exist before anything can play, and it
  // outlives the widget tree — that is what keeps music going when the app is
  // backgrounded and the screen is locked.
  final audioHandler = await initAudioService();

  runApp(
    ProviderScope(
      overrides: <Override>[
        audioHandlerProvider.overrideWithValue(audioHandler),
      ],
      child: const AtticApp(),
    ),
  );
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

  @override
  void initState() {
    super.initState();
    // Restore a saved session before the first frame settles, so a returning
    // user lands in their library rather than on the login screen.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(authControllerProvider.notifier).restore();
    });
  }

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
          // Read, not watched: rebuilding the router would reset navigation
          // state every time the theme changes.
          routerConfig: ref.read(routerProvider),
        );
      },
    );
  }
}
