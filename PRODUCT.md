# Product

## Register

product

## Users

Administrators monitor several potted plants through fixed ESP32 sensor nodes. They use the dashboard primarily on a desktop in a bright working environment to compare global sensor trends and identify changes that warrant attention. Mobile access supports quick reading and filtering rather than dense analysis.

## Product Purpose

The dashboard turns five-minute pH, EC, light, and soil-moisture readings into a trustworthy global view of plant conditions. Its main task is comparative trend analysis: a user opens the dashboard to inspect a selected metric across relevant plants over the last 24 hours, then changes the time range or plant selection to investigate further.

## Brand Personality

Direct, reliable, operational. The interface follows Uber Base / Base Web closely: familiar controls, explicit state, deliberate density, and accessibility are more important than decorative expression. Plant identity belongs in labels and data, not in ornamental visuals.

## Anti-references

- Botanical or gardening-brand decoration, illustrations, and ornamental green surfaces.
- Glassmorphism, gradient treatments, or a stack of oversized decorative cards.
- A multicolored dashboard that mixes unrelated units in one visual encoding.
- A loose imitation of Uber styling in place of actual Base Web components and behavior.

## Design Principles

1. Make the global comparison the first thing the user can read.
2. Represent one metric per chart so units and trends remain honest.
3. Preserve Base Web defaults and accessible behavior; use overrides only when the data task requires them.
4. Let data density serve investigation, with deliberate filtering instead of visual clutter.
5. Give state changes immediate, restrained feedback and keep motion out of analytical flow.

## Accessibility & Inclusion

Meet WCAG 2.2 AA. Do not communicate sensor or alert status by color alone. Provide text alternatives or summaries for chart state, keyboard-accessible filters, visible focus treatment, and responsive reflow. Respect `prefers-reduced-motion` by removing chart-drawing and spatial motion, retaining only short opacity feedback; respect higher-contrast preferences. Use a light theme by default and offer a dark alternative for low-light work.
