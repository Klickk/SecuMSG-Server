import { useState } from "react";
import { Route, Routes } from "react-router-dom";
import { AuthPage } from "./components/AuthPage";

const App = () => {
  const [loggedIn, setLoggedIn] = useState<boolean>(false);

  return (
    <div>
      <Routes>
        {loggedIn ? (
          <></>
        ) : (
          // Render logged-in routes/components here
          <Route path="/" Component={AuthPage} />
          // Render login/signup routes/components here
        )}
      </Routes>
    </div>
  );
};

export default App;
