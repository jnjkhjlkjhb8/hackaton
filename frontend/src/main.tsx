import { StrictMode, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { BaseProvider, DarkTheme, LightTheme } from "baseui";
import { Client as Styletron } from "styletron-engine-monolithic";
import { Provider as StyletronProvider } from "styletron-react";

import App from "./App";
import "./styles.css";

const engine = new Styletron();

function Root() {
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    const canvas = isDark ? "#000000" : "#ffffff";
    document.documentElement.style.backgroundColor = canvas;
    document.body.style.backgroundColor = canvas;
  }, [isDark]);

  return (
    <StyletronProvider value={engine}>
      <BaseProvider theme={isDark ? DarkTheme : LightTheme}>
        <App isDark={isDark} onToggleTheme={() => setIsDark((current) => !current)} />
      </BaseProvider>
    </StyletronProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Root />
  </StrictMode>,
);
