import { Route, Routes } from "react-router-dom";
import { AuthPage } from "./components/AuthPage";
import { DeviceRegisterForm } from "./components/DeviceRegister";
import { MessagingPage } from "./components/MessagingPage";
import { ProfilePage } from "./components/ProfilePage";

const App = () => {
  return (
    <div>
      <Routes>
        <Route path="/" Component={AuthPage} />
        <Route path="/dRegister" Component={DeviceRegisterForm} />
        <Route path="/messages" Component={MessagingPage} />
        <Route path="/profile" Component={ProfilePage} />
      </Routes>
    </div>
  );
};

export default App;
