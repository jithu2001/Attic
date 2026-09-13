import 'package:flutter/cupertino.dart' show CupertinoPageTransitionsBuilder;
import 'package:flutter/material.dart';

/// Attic's Material 3 theme.
///
/// The whole app is M3 (`useMaterial3: true`). Colour always comes from the
/// device's dynamic palette when Android 12+ provides one, and falls back to a
/// seeded scheme otherwise — see [AtticApp]'s `DynamicColorBuilder`.
///
/// Widgets must never hardcode colours: read roles off
/// `Theme.of(context).colorScheme`, and text styles off
/// `Theme.of(context).textTheme`.
class AppTheme {
  const AppTheme._();

  /// Seed colour used when the platform has no dynamic palette.
  /// M3 baseline purple.
  static const Color seed = Color(0xFF6750A4);

  static ThemeData light(ColorScheme? dynamicScheme) =>
      _build(dynamicScheme ?? ColorScheme.fromSeed(seedColor: seed));

  static ThemeData dark(ColorScheme? dynamicScheme) => _build(
        dynamicScheme ??
            ColorScheme.fromSeed(seedColor: seed, brightness: Brightness.dark),
      );

  /// Media-heavy surfaces (video player, photo viewer) deliberately depart from
  /// the tonal surface palette: pure black keeps attention on the content and
  /// saves power on OLED panels. This is the *only* sanctioned exception to the
  /// "colours come from colorScheme" rule, and those screens opt in explicitly
  /// by wrapping themselves in [immersive].
  static ThemeData immersive(ThemeData base) => base.copyWith(
        scaffoldBackgroundColor: Colors.black,
        colorScheme: base.colorScheme.copyWith(surface: Colors.black),
      );

  static ThemeData _build(ColorScheme scheme) {
    final base = ThemeData(useMaterial3: true, colorScheme: scheme);

    return base.copyWith(
      // M3 defaults are correct for shape and elevation; only set what the
      // design system asks for beyond them.
      appBarTheme: AppBarTheme(
        backgroundColor: scheme.surface,
        surfaceTintColor: scheme.surfaceTint,
        centerTitle: false,
      ),
      navigationBarTheme: NavigationBarThemeData(
        backgroundColor: scheme.surfaceContainer,
        elevation: 3,
      ),
      pageTransitionsTheme: const PageTransitionsTheme(
        builders: <TargetPlatform, PageTransitionsBuilder>{
          // Predictive back on Android 14+, falling back gracefully below it.
          TargetPlatform.android: PredictiveBackPageTransitionsBuilder(),
          TargetPlatform.iOS: CupertinoPageTransitionsBuilder(),
        },
      ),
    );
  }
}
