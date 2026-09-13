import 'package:attic/core/theme/app_theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('themes are Material 3 and seeded when no dynamic palette exists', () {
    final light = AppTheme.light(null);
    final dark = AppTheme.dark(null);

    expect(light.useMaterial3, isTrue);
    expect(dark.useMaterial3, isTrue);
    expect(light.colorScheme.brightness, Brightness.light);
    expect(dark.colorScheme.brightness, Brightness.dark);
  });

  test('a dynamic palette overrides the seed', () {
    final dynamicScheme = ColorScheme.fromSeed(seedColor: const Color(0xFF00695C));
    final theme = AppTheme.light(dynamicScheme);

    expect(theme.colorScheme.primary, dynamicScheme.primary);
    expect(theme.colorScheme.primary, isNot(AppTheme.light(null).colorScheme.primary));
  });

  test('immersive surfaces are pure black for media playback', () {
    final immersive = AppTheme.immersive(AppTheme.dark(null));

    expect(immersive.scaffoldBackgroundColor, Colors.black);
    expect(immersive.colorScheme.surface, Colors.black);
  });
}
