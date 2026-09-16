import type { LOCALES } from "./config";
import type en from "../../messages/en.json";

// Type the message catalogue after English so `t("missing.key")` is a
// compile error and every locale file must carry the same shape.
declare module "next-intl" {
  interface AppConfig {
    Locale: (typeof LOCALES)[number];
    Messages: typeof en;
  }
}
