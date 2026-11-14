import type { Transition, Variants } from "framer-motion";

export const cartSpring: Transition = { type: "spring", stiffness: 180, damping: 24, mass: 0.8 };

export const listContainerVariants: Variants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.035 } },
};

export const listItemVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.2, ease: [0.16, 1, 0.3, 1] } },
  exit: { opacity: 0, x: -10, transition: { duration: 0.16 } },
};
