import { getRequestConfig } from "next-intl/server";
import { cookies, headers } from "next/headers";
import { isLocale, LOCALE_COOKIE, negotiateLocale, type Locale } from "./config";

/**
 * Resolve the request's locale: an explicit cookie (set by the switcher)
 * wins, otherwise the browser's Accept-Language. Reading request state here
 * makes every page dynamic — accepted; nothing in the app was relying on
 * static prerendering, and the alternative (locale in the URL) is the
 * restructure config.ts explains we're avoiding.
 */
export default getRequestConfig(async () => {
  const cookieLocale = (await cookies()).get(LOCALE_COOKIE)?.value;
  const locale: Locale = isLocale(cookieLocale)
    ? cookieLocale
    : negotiateLocale((await headers()).get("accept-language"));

  return {
    locale,
    messages: (await import(`../../messages/${locale}.json`)).default,
  };
});
