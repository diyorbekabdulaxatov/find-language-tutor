-- Backend i18n: the language a person wants to be addressed in. Error
-- responses use the request's Accept-Language; email has no request, so it
-- uses this. Set from Accept-Language at registration and changed via
-- PATCH /v1/auth/me when the person picks a language in the UI.
ALTER TABLE users
    ADD COLUMN locale text NOT NULL DEFAULT 'en'
        CHECK (locale IN ('en', 'ru', 'uz'));
