import 'package:attic/core/permissions/notification_permission.dart';
import 'package:attic/core/theme/app_theme.dart';
import 'package:attic/features/settings/settings_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'notification_permission_test.dart' show FakePlatform;

Future<void> pumpCard(WidgetTester tester, NotificationAccess access) async {
  await tester.pumpWidget(ProviderScope(
    overrides: <Override>[
      notificationPermissionPlatformProvider
          .overrideWithValue(FakePlatform(initial: access)),
    ],
    child: MaterialApp(
      theme: AppTheme.light(null),
      home: const Scaffold(body: NotificationPermissionSection()),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('says nothing when permission is granted', (tester) async {
    // A settings screen full of resolved warnings trains people to ignore them.
    await pumpCard(tester, NotificationAccess.granted);

    expect(find.text('Lock-screen controls'), findsNothing);
    expect(tester.getSize(find.byType(NotificationPermissionCard)).height, 0);
    // And the section heading goes with it: a heading over empty space reads
    // as a rendering bug.
    expect(find.text('Playback'), findsNothing);
  });

  testWidgets('shows the section heading only alongside the card',
      (tester) async {
    await pumpCard(tester, NotificationAccess.askable);
    expect(find.text('Playback'), findsOneWidget);
  });

  testWidgets('offers to ask when the system will still prompt', (tester) async {
    await pumpCard(tester, NotificationAccess.askable);

    expect(find.text('Lock-screen controls'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Allow notifications'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Open settings'), findsNothing);
    // The wording has to make clear that refusing does not break playback.
    expect(find.textContaining('Music plays either way'), findsOneWidget);
  });

  testWidgets('sends the user to settings once the system has stopped asking',
      (tester) async {
    await pumpCard(tester, NotificationAccess.blocked);

    expect(find.widgetWithText(FilledButton, 'Open settings'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Allow notifications'), findsNothing);
  });

  testWidgets('asking updates the card in place', (tester) async {
    await pumpCard(tester, NotificationAccess.askable);

    await tester.tap(find.widgetWithText(FilledButton, 'Allow notifications'));
    await tester.pumpAndSettle();

    // Granted, so the card takes itself away.
    expect(find.text('Lock-screen controls'), findsNothing);
  });
}
