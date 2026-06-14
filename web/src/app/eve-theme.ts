import { definePreset } from '@primeuix/themes';
import Aura from '@primeuix/themes/aura';

// EVE Online "Photon UI" inspired theme:
// near-black space backgrounds, sharp angular corners, teal/cyan primary,
// amber secondary, compact typography.
export const EvePreset = definePreset(Aura, {
  primitive: {
    // EVE teal/cyan
    eveTeal: {
      50: '#e6f4f7',
      100: '#c0e4ec',
      200: '#96d2e0',
      300: '#69bed3',
      400: '#46afc9',
      500: '#4da8c0',
      600: '#3d95ae',
      700: '#2d7f96',
      800: '#1d697e',
      900: '#0e5366',
      950: '#07374a',
    },
  },
  semantic: {
    primary: {
      50: '{eveTeal.50}',
      100: '{eveTeal.100}',
      200: '{eveTeal.200}',
      300: '{eveTeal.300}',
      400: '{eveTeal.400}',
      500: '{eveTeal.500}',
      600: '{eveTeal.600}',
      700: '{eveTeal.700}',
      800: '{eveTeal.800}',
      900: '{eveTeal.900}',
      950: '{eveTeal.950}',
    },
    borderRadius: {
      none: '0',
      xs: '1px',
      sm: '1px',
      md: '2px',
      lg: '2px',
      xl: '3px',
    },
    colorScheme: {
      light: {
        primary: {
          color: '{eveTeal.600}',
          contrastColor: '#ffffff',
          hoverColor: '{eveTeal.700}',
          activeColor: '{eveTeal.800}',
        },
        highlight: {
          background: 'rgba(45,127,150,0.12)',
          focusBackground: 'rgba(45,127,150,0.20)',
          color: '{eveTeal.800}',
          focusColor: '{eveTeal.900}',
        },
        surface: {
          0: '#ffffff',
          50: '#f5f7f9',
          100: '#eef1f4',
          200: '#e4e9ed',
          300: '#d7dee3',
          400: '#c4ccd3',
          500: '#aab4bc',
          600: '#8b96a0',
          700: '#6c7884',
          800: '#4f5a64',
          900: '#38424b',
          950: '#232b32',
        },
        text: {
          color: '#1b2228',
          hoverColor: '#0d1216',
          mutedColor: '#5c6770',
        },
        content: {
          borderColor: '#d2dae0',
        },
      },
      dark: {
        primary: {
          color: '{eveTeal.500}',
          contrastColor: '#ffffff',
          hoverColor: '{eveTeal.400}',
          activeColor: '{eveTeal.600}',
        },
        highlight: {
          background: 'rgba(77,168,192,0.12)',
          focusBackground: 'rgba(77,168,192,0.20)',
          color: '{eveTeal.400}',
          focusColor: '{eveTeal.300}',
        },
        surface: {
          0: '#0b0e11',
          50: '#0f1215',
          100: '#131619',
          200: '#171b1f',
          300: '#1c2024',
          400: '#21262c',
          500: '#282e36',
          600: '#353c45',
          700: '#454d57',
          800: '#5c6470',
          900: '#7a8290',
          950: '#9da4ac',
        },
        text: {
          color: '#c2c6ca',
          hoverColor: '#d0d4d8',
          mutedColor: '#6e7680',
        },
        content: {
          // Aura's default content.background is {surface.900}, which is bright
          // under the inverted ramp (e.g. the datepicker panel). Match it to the
          // form-field fill so overlays read as dark.
          background: '{surface.300}',
          hoverBackground: '{surface.400}',
          borderColor: '#232830',
          color: '{text.color}',
          hoverColor: '{text.hover.color}',
        },
        // EVE's dark ramp inverts brightness (surface.0 is darkest), so Aura's
        // defaults — formField {surface.950}, overlay {surface.900} — resolve to
        // bright fills. Re-point them at the dark end with light text so input
        // fields, the datepicker, popovers and select panels read as dark.
        formField: {
          background: '{surface.300}',
          filledBackground: '{surface.300}',
          filledHoverBackground: '{surface.400}',
          filledFocusBackground: '{surface.400}',
          borderColor: '{surface.600}',
          hoverBorderColor: '{surface.700}',
          color: '{text.color}',
          placeholderColor: '{text.muted.color}',
          floatLabelColor: '{text.muted.color}',
          iconColor: '{text.muted.color}',
        },
        overlay: {
          popover: {
            background: '{surface.300}',
            borderColor: '{surface.600}',
            color: '{text.color}',
          },
          select: {
            background: '{surface.300}',
            borderColor: '{surface.600}',
            color: '{text.color}',
          },
        },
      },
    },
  },
  components: {
    // Aura's secondary-button tokens reference {surface.800}/{surface.300} expecting
    // light-ish fills; EVE's dark ramp inverts brightness (surface.0 is darkest), so
    // we re-point the dark secondary tokens at the correct shades here instead of
    // overriding rendered buttons with global !important rules.
    button: {
      colorScheme: {
        dark: {
          root: {
            secondary: {
              background: '{surface.300}',
              hoverBackground: '{surface.400}',
              activeBackground: '{surface.400}',
              borderColor: '{surface.600}',
              hoverBorderColor: '{surface.600}',
              activeBorderColor: '{surface.600}',
              color: '{text.color}',
              hoverColor: '{text.hover.color}',
              activeColor: '{text.hover.color}',
            },
          },
          text: {
            secondary: {
              color: '{text.color}',
              hoverBackground: '{surface.200}',
              activeBackground: '{surface.300}',
            },
          },
        },
      },
    },
    tooltip: {
      root: {
        borderRadius: '{borderRadius.xs}',
      },
      colorScheme: {
        // Light mode: a dark tooltip with light text.
        light: {
          root: {
            background: '{surface.950}',
            color: '{surface.0}',
          },
        },
        // Dark mode: a slightly-elevated dark fill with light text, so it lifts
        // off the panels without going bright.
        dark: {
          root: {
            background: '{surface.400}',
            color: '{text.color}',
          },
        },
      },
    },
    // Popover colors come from the semantic overlay.popover tokens above; only
    // the structural tweaks (sharp corners, zero padding) are set here.
    popover: {
      root: {
        borderRadius: '{borderRadius.xs}',
      },
      content: {
        padding: '0',
      },
    },
  },
});
