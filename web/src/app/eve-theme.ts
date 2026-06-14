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
      },
    },
  },
});
