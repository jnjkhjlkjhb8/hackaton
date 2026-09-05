<!-- SEED: re-run $impeccable document once there's code to capture the actual tokens and components. -->
---
name: Plant Telemetry Dashboard
description: A Base Web dashboard for comparing global plant-sensor trends.
---

# Design System: Plant Telemetry Dashboard

## Overview

**Creative North Star: "The Base Operations Console"**

This is a direct, reliable, operational interface for reading sensor data—not a gardening brand experience. The dashboard adopts Uber Base / Base Web as its only visual and component source. Its first task is to make one metric comparable across relevant plants in a 24-hour global chart; filtering and drill-in exist to sharpen that reading, never to decorate it.

The system is desktop-first in a bright work environment. It is compact enough for comparative analysis, but never cramped: clear labels, explicit units, keyboard-accessible controls, and visible state always outrank visual novelty. Mobile reflows into a focused reading and filtering experience rather than attempting to preserve desktop density.

**Key Characteristics:**

- Familiar Base Web controls and state behavior, used without stylistic imitation.
- Restrained light surfaces with semantic data color only where it carries meaning.
- One metric per chart; comparisons never mix pH, EC, light, and moisture scales.
- Immediate, quiet interaction feedback; analysis is never delayed by choreography.

## Colors

The palette is intentionally restrained: Base Web's implemented light theme is the source of truth once the frontend exists; semantic and chart colors explain data, not brand personality.

### Primary

- **Base Action Ink** ([to be resolved from the implemented Base Web theme]): Primary actions, selected navigation, and high-confidence interactive state. It is never used as surface decoration.

### Secondary

- **Semantic Signal Set** ([to be resolved from the implemented Base Web theme]): Success, warning, error, and information communicate state alongside text and icon labels. Chart series colors must remain distinguishable without relying on hue alone.

### Neutral

- **Base Light Surfaces** ([to be resolved from the implemented Base Web theme]): The default working canvas, panels, dividers, and text hierarchy. Do not introduce a botanical green or a tinted paper background.

**The Base-Is-Source Rule.** Do not invent a brand palette. Import Base Web's light theme and use its tokens; only add semantic chart mappings after contrast and non-color identification are verified.

## Typography

**Display Font:** None. Product headings use the same Base Web sans-serif hierarchy as the interface.

**Body Font:** Base Web system sans-serif stack ([font token to be captured after implementation]).

**Label/Mono Font:** Use the body family for labels and numeric data until Base Web exposes an implemented alternative.

**Character:** Clear, compact, and neutral. Typography differentiates labels, values, chart legends, and controls by Base Web's real hierarchy—not by expressive font pairing.

### Hierarchy

- **Headline** (Base Web implemented heading token): Page and panel orientation only; never marketing-scale.
- **Title** (Base Web implemented title token): Chart title, selected metric, and important contextual state.
- **Body** (Base Web implemented body token): Explanations and chart summaries; prose stays within 65–75ch where it occurs.
- **Label** (Base Web implemented label token): Controls, units, legends, timestamps, and compact metadata. Labels remain sentence case unless the component library already defines otherwise.

**The One-Family Rule.** Do not add a display, serif, mono-forward, or hand-written typeface to create a plant mood. The data is the personality.

## Elevation

The interface is flat by default, using Base Web's implemented surface and border hierarchy to separate regions. Elevation may only clarify an overlay, menu, focused control, or temporarily lifted interaction; ambient card shadows and translucent decoration are forbidden.

**The Evidence-Only Elevation Rule.** If a shadow does not prove spatial priority or an active interaction, remove it.

## Components

Component details will be captured from the actual `baseui` implementation on the next scan pass. Until then, the following usage rules are binding.

### Buttons

- **Style:** Use Base Web button variants directly. Primary actions are scarce and task-specific; filters and non-destructive utilities use the appropriate secondary or tertiary Base variant.
- **Feedback:** Provide press feedback immediately. Pointer actions may use a short 150–200ms state transition; keyboard-initiated actions do not wait for decorative motion.

### Inputs / Fields

- **Style:** Use Base Web select, input, checkbox, and multi-select controls rather than custom chart filters.
- **Focus:** Retain Base Web's visible, keyboard-accessible focus behavior. Every selected or unavailable state must have a textual cue.

### Navigation

- **Style:** Keep navigation concise and task-oriented. The dashboard route leads with the global chart; navigation must not compete with the data canvas.
- **Responsive behavior:** On narrow viewports, prioritize the current page, the metric selector, and the date range over persistent desktop navigation.

### Global Trend Chart

- **Style:** Display one selected metric at a time, defaulting to soil moisture. Default range is 24 hours with 7-day and 30-day controls.
- **Selection:** Show only plants requiring attention or explicitly pinned by the user; expose an accessible searchable multi-select for all plants.
- **Accessibility:** Provide a text summary of the visible range, selected plants, latest reading, and exceptional values. Do not encode a plant identity or status with color alone.
- **Motion:** Updating data crossfades or changes instantly when reduced motion is requested; it never performs a decorative line-drawing animation.

## Do's and Don'ts

### Do:

- **Do** import actual Base Web components and its light theme before making an override.
- **Do** compare one metric per chart and show units adjacent to the title and values.
- **Do** use semantic state with an icon or text label as well as color, meeting WCAG 2.2 AA.
- **Do** keep analysis controls keyboard reachable and preserve visible focus treatment.
- **Do** respect reduced motion with an instant update or short opacity-only change.

### Don't:

- **Don't** use botanical or gardening-brand decoration, illustrations, or ornamental green surfaces.
- **Don't** use glassmorphism, gradients, or stacks of oversized decorative cards.
- **Don't** mix unrelated pH, EC, light, and soil-moisture units in one chart encoding.
- **Don't** imitate Uber Base with custom lookalike components; use Base Web behavior and tokens.
- **Don't** let the dashboard resemble a Notion-style document or template page with a cover, airy hero, or decorative plant motif.
