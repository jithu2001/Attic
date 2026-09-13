import 'package:attic/core/layout/app_shell.dart';
import 'package:attic/core/layout/window_size.dart';
import 'package:attic/core/theme/app_theme.dart';
import 'package:attic/features/music/player/mini_player.dart';
import 'package:attic/features/music/player/player_controller.dart';
import 'package:audio_service/audio_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// Pumps the shell with the player's state stubbed out.
///
/// The mini-player is part of the shell, so the shell cannot be tested without
/// deciding what is playing; overriding the two derived providers avoids
/// standing up a real background audio service in a unit test.
Future<void> pumpShell(
  WidgetTester tester, {
  required Size size,
  int selectedIndex = 0,
  ValueChanged<int>? onDestinationSelected,
  MediaItem? nowPlaying,
  PlaybackState? playbackState,
}) async {
  // The view, not MediaQuery: a wrapped MediaQuery would lie to the widgets
  // about the window while the real viewport still clipped at 800x600, so the
  // layout under test would not be the one being asserted.
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);

  await tester.pumpWidget(ProviderScope(
    overrides: <Override>[
      currentMediaItemProvider.overrideWith((ref) => Stream.value(nowPlaying)),
      playbackStateProvider.overrideWith(
        (ref) => Stream.value(playbackState ?? PlaybackState()),
      ),
    ],
    child: MaterialApp(
      theme: AppTheme.light(null),
      home: AppShell(
        selectedIndex: selectedIndex,
        onDestinationSelected: onDestinationSelected ?? (_) {},
        child: const SizedBox(),
      ),
    ),
  ));
  // Two frames: one to mount, one for the stubbed playback streams to deliver
  // their first value.
  await tester.pump();
  await tester.pump();
}

void main() {
  const phone = Size(400, 800);
  const desktop = Size(1280, 800);

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
    await pumpShell(tester, size: phone);

    expect(find.byType(NavigationBar), findsOneWidget);
    expect(find.byType(NavigationRail), findsNothing);
    expect(find.byType(NavigationDestination), findsNWidgets(4));
    for (final label in <String>['Photos', 'Music', 'Video', 'Settings']) {
      expect(find.text(label), findsOneWidget);
    }
  });

  testWidgets('expanded windows get a NavigationRail instead', (tester) async {
    await pumpShell(tester, size: desktop, selectedIndex: 1);

    expect(find.byType(NavigationRail), findsOneWidget);
    expect(find.byType(NavigationBar), findsNothing);
  });

  testWidgets('tapping a destination reports its index', (tester) async {
    int? tapped;
    await pumpShell(tester, size: phone, onDestinationSelected: (i) => tapped = i);

    await tester.tap(find.text('Settings'));
    expect(tapped, 3);
  });

  testWidgets('the mini-player takes no space when nothing is playing',
      (tester) async {
    await pumpShell(tester, size: phone);

    expect(find.byType(MiniPlayer), findsOneWidget);
    // Present in the tree but zero-height: the shell must not reserve room for
    // a player that does not exist.
    expect(tester.getSize(find.byType(MiniPlayer)).height, 0);
    expect(find.byIcon(Icons.play_arrow), findsNothing);
  });

  testWidgets('the mini-player shows the current track above the nav bar',
      (tester) async {
    await pumpShell(
      tester,
      size: phone,
      nowPlaying: const MediaItem(
        id: 'https://homeserver.tailnet.ts.net/api/v1/music/tracks/1/audio',
        title: 'So What',
        artist: 'Miles Davis',
        duration: Duration(minutes: 9, seconds: 22),
      ),
      playbackState: PlaybackState(playing: false),
    );

    expect(find.text('So What'), findsOneWidget);
    expect(find.text('Miles Davis'), findsOneWidget);
    // Paused, so the control offers play.
    expect(find.byIcon(Icons.play_arrow), findsOneWidget);

    final playerBottom = tester.getRect(find.byType(MiniPlayer)).bottom;
    final navTop = tester.getRect(find.byType(NavigationBar)).top;
    expect(playerBottom, lessThanOrEqualTo(navTop),
        reason: 'the mini-player is docked above the navigation bar');
  });

  testWidgets('the mini-player offers pause while playing', (tester) async {
    await pumpShell(
      tester,
      size: phone,
      nowPlaying: const MediaItem(id: 'x', title: 'So What', artist: 'Miles Davis'),
      playbackState: PlaybackState(playing: true),
    );

    expect(find.byIcon(Icons.pause), findsOneWidget);
    expect(find.byIcon(Icons.play_arrow), findsNothing);
  });
}
