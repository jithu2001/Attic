import 'package:attic/core/layout/app_shell.dart';
import 'package:attic/core/layout/window_size.dart';
import 'package:attic/core/theme/app_theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _harness({required Size size, required Widget child}) {
  return MediaQuery(
    data: MediaQueryData(size: size),
    child: MaterialApp(theme: AppTheme.light(null), home: child),
  );
}

void main() {
  group('WindowSizeClass', () {
    test('classifies M3 breakpoints', () {
      expect(WindowSizeClass.fromWidth(400), WindowSizeClass.compact);
      expect(WindowSizeClass.fromWidth(599.9), WindowSizeClass.compact);
      expect(WindowSizeClass.fromWidth(600), WindowSizeClass.medium);
      expect(WindowSizeClass.fromWidth(839.9), WindowSizeClass.medium);
      expect(WindowSizeClass.fromWidth(840), WindowSizeClass.expanded);
    });

    test('uses 16 dp margins when compact and 24 dp beyond', () {
      expect(WindowSizeClass.compact.screenMargin, 16);
      expect(WindowSizeClass.medium.screenMargin, 24);
      expect(WindowSizeClass.expanded.screenMargin, 24);
    });
  });

  testWidgets('compact windows get a NavigationBar with four destinations',
      (tester) async {
    await tester.pumpWidget(_harness(
      size: const Size(400, 800),
      child: AppShell(
        selectedIndex: 0,
        onDestinationSelected: (_) {},
        child: const SizedBox(),
      ),
    ));

    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(NavigationRail), findsNothing);
    expect(find.byType(NavigationDestination), findsNWidgets(4));
    for (final label in <String>['Photos', 'Music', 'Video', 'Settings']) {
      expect(find.text(label), findsOneWidget);
    }
  });

  testWidgets('expanded windows get a NavigationRail instead', (tester) async {
    await tester.pumpWidget(_harness(
      size: const Size(1280, 800),
      child: AppShell(
        selectedIndex: 1,
        onDestinationSelected: (_) {},
        child: const SizedBox(),
      ),
    ));

    expect(find.byType(NavigationRail), findsOneWidget);
    expect(find.byType(NavigationBar), findsNothing);
  });

  testWidgets('tapping a destination reports its index', (tester) async {
    int? tapped;
    await tester.pumpWidget(_harness(
      size: const Size(400, 800),
      child: AppShell(
        selectedIndex: 0,
        onDestinationSelected: (i) => tapped = i,
        child: const SizedBox(),
      ),
    ));

    await tester.tap(find.text('Settings'));
    expect(tapped, 3);
  });
}
