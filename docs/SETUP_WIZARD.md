# Setup wizard

The wizard presents editable steps with Back/Next navigation and a final Start action. Desktop windows with enough width and height show an editable step sidebar; compact windows use a single column with fixed navigation. Name, persona and appearance selections stay in wizard state when navigating. Mobile builds retain the server address/token step first. Preferences can be changed later in Settings.

The model connection step offers three paths:

- **Local model:** inspect hardware recommendations and download the existing curated chat and embedding models. An embedding model alone does not count as a ready chat model.
- **API key:** open the existing provider dialog. A newly added connection is activated using the existing provider flow.
- **Subscription:** use the existing account sign-in, callback-paste, cancellation and model refresh flow inline. Once account models are listed, choose **Use subscription** to activate the Subscriptions provider for chat. Sign-in alone does not switch the chat provider. Activation errors stay visible inline.

API and subscription users can independently download the local embedding model for memory. Model downloads are optional; the final system check shows readiness and missing connections without forcing a download. A missing local chat model is neutral when an external chat provider is selected.

Widget tests cover Turkish and English at 360x900, 1100x900 and 1100x600, subscription sign-in through the existing UI, waiting for account models, explicit activation, name preservation and persisted setup completion. A separate mobile test covers the server step and back navigation. Tests use a fake API; live vendor login and the native desktop window were not exercised in this change.
