import 'package:flutter/widgets.dart';

/// Material 3 window size classes.
///
/// Drives the whole adaptive story: compact gets a `NavigationBar` and a single
/// pane, medium and expanded get a `NavigationRail` and may show two panes.
enum WindowSizeClass {
  compact,
  medium,
  expanded;

  /// Classify a width in logical pixels per the M3 breakpoints.
  static WindowSizeClass fromWidth(double width) {
    if (width < 600) return WindowSizeClass.compact;
    if (width < 840) return WindowSizeClass.medium;
    return WindowSizeClass.expanded;
  }

  static WindowSizeClass of(BuildContext context) =>
      fromWidth(MediaQuery.sizeOf(context).width);

  bool get isCompact => this == WindowSizeClass.compact;

  /// Screen margin from the M3 layout spec: 16 dp compact, 24 dp beyond.
  double get screenMargin => isCompact ? 16 : 24;
}
